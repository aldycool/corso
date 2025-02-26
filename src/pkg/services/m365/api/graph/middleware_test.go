package graph

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/alcionai/clues"
	"github.com/google/uuid"
	khttp "github.com/microsoft/kiota-http-go"
	msgraphsdkgo "github.com/microsoftgraph/msgraph-sdk-go"
	msgraphgocore "github.com/microsoftgraph/msgraph-sdk-go-core"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/alcionai/corso/src/internal/common/limiters"
	"github.com/alcionai/corso/src/internal/common/ptr"
	"github.com/alcionai/corso/src/internal/tester"
	"github.com/alcionai/corso/src/internal/tester/tconfig"
	"github.com/alcionai/corso/src/pkg/account"
	"github.com/alcionai/corso/src/pkg/count"
	"github.com/alcionai/corso/src/pkg/path"
	graphTD "github.com/alcionai/corso/src/pkg/services/m365/api/graph/testdata"
)

type mwReturns struct {
	err  error
	resp *http.Response
}

func newMWReturns(code int, body []byte, err error) mwReturns {
	var brc io.ReadCloser

	if len(body) > 0 {
		brc = io.NopCloser(bytes.NewBuffer(body))
	}

	resp := &http.Response{
		ContentLength: int64(len(body)),
		StatusCode:    code,
		Body:          brc,
	}

	if code == 0 {
		resp = nil
	}

	return mwReturns{
		err:  err,
		resp: resp,
	}
}

func newTestMW(onIntercept func(*http.Request), mrs ...mwReturns) *testMW {
	return &testMW{
		onIntercept: onIntercept,
		toReturn:    mrs,
	}
}

type testMW struct {
	repeatReturn0 bool
	iter          int
	toReturn      []mwReturns
	onIntercept   func(*http.Request)
}

func (mw *testMW) Intercept(
	pipeline khttp.Pipeline,
	middlewareIndex int,
	req *http.Request,
) (*http.Response, error) {
	mw.onIntercept(req)

	i := mw.iter
	if mw.repeatReturn0 {
		i = 0
	}

	if i >= len(mw.toReturn) {
		panic(clues.New("middleware test had more calls than responses"))
	}

	tr := mw.toReturn[i]

	mw.iter++

	return tr.resp, tr.err
}

// can't use graph/mock.CreateAdapter() due to circular references.
func mockAdapter(
	creds account.M365Config,
	mw khttp.Middleware,
	cc *clientConfig,
) (*msgraphsdkgo.GraphRequestAdapter, error) {
	auth, err := GetAuth(
		creds.AzureTenantID,
		creds.AzureClientID,
		creds.AzureClientSecret,
		creds.AzureOnBehalfOfRefreshToken,
		creds.AzureOnBehalfOfServiceID,
		creds.AzureOnBehalfOfServiceSecret)
	if err != nil {
		return nil, err
	}

	var (
		clientOptions = msgraphsdkgo.GetDefaultClientOptions()
		middlewares   = append(kiotaMiddlewares(&clientOptions, cc, count.New()), mw)
		httpClient    = msgraphgocore.GetDefaultClient(&clientOptions, middlewares...)
	)

	cc.apply(httpClient)

	return msgraphsdkgo.NewGraphRequestAdapterWithParseNodeFactoryAndSerializationWriterFactoryAndHttpClient(
		auth,
		nil, nil,
		httpClient)
}

type RetryMWIntgSuite struct {
	tester.Suite
	creds account.M365Config
}

// We do end up mocking the actual request, but creating the rest
// similar to E2E suite
func TestRetryMWIntgSuite(t *testing.T) {
	suite.Run(t, &RetryMWIntgSuite{
		Suite: tester.NewIntegrationSuite(
			t,
			[][]string{tconfig.M365AcctCredEnvs}),
	})
}

func (suite *RetryMWIntgSuite) SetupSuite() {
	var (
		a   = tconfig.NewM365Account(suite.T())
		err error
	)

	ctx, flush := tester.NewContext(suite.T())
	defer flush()

	InitializeConcurrencyLimiter(ctx, false, -1)

	suite.creds, err = a.M365Config()
	require.NoError(suite.T(), err, clues.ToCore(err))
}

func (suite *RetryMWIntgSuite) TestRetryMiddleware_Intercept_byStatusCode() {
	var (
		uri     = "https://graph.microsoft.com"
		urlPath = "/v1.0/users/user/messages/foo"
		url     = uri + urlPath
	)

	tests := []struct {
		name             string
		status           int
		providedErr      error
		expectRetryCount int
		mw               testMW
		expectErr        assert.ErrorAssertionFunc
	}{
		{
			name:             "200, no retries",
			status:           http.StatusOK,
			providedErr:      nil,
			expectRetryCount: 0,
			expectErr:        assert.NoError,
		},
		{
			name:             "400, no retries",
			status:           http.StatusBadRequest,
			providedErr:      nil,
			expectRetryCount: 0,
			expectErr:        assert.Error,
		},
		{
			name:             "502",
			status:           http.StatusBadGateway,
			providedErr:      nil,
			expectRetryCount: defaultMaxRetries,
			expectErr:        assert.Error,
		},
		// 503 and 504 retries are handled by kiota retry handler. Adding
		// tests here to ensure we don't regress on retrying these errors.
		// Configure retry count to 1 so that the test case doesn't run for too
		// long due to exponential backoffs.
		{
			name:             "503",
			status:           http.StatusServiceUnavailable,
			providedErr:      nil,
			expectRetryCount: 1,
			expectErr:        assert.Error,
		},
		{
			name:             "504",
			status:           http.StatusGatewayTimeout,
			providedErr:      nil,
			expectRetryCount: 1,
			expectErr:        assert.Error,
		},
		{
			name:             "conn reset with 5xx",
			status:           http.StatusBadGateway,
			providedErr:      syscall.ECONNRESET,
			expectRetryCount: defaultMaxRetries,
			expectErr:        assert.Error,
		},
		{
			name:             "conn reset with 2xx",
			status:           http.StatusOK,
			providedErr:      syscall.ECONNRESET,
			expectRetryCount: defaultMaxRetries,
			expectErr:        assert.Error,
		},
		{
			name:        "conn reset with nil resp",
			providedErr: syscall.ECONNRESET,
			// Use 0 to denote nil http response
			status:           0,
			expectRetryCount: 3,
			expectErr:        assert.Error,
		},
		{
			// Unlikely but check if connection reset error takes precedence
			name:             "conn reset with 400 resp",
			providedErr:      syscall.ECONNRESET,
			status:           http.StatusBadRequest,
			expectRetryCount: 3,
			expectErr:        assert.Error,
		},
		{
			name:             "http timeout",
			providedErr:      http.ErrHandlerTimeout,
			status:           0,
			expectRetryCount: 3,
			expectErr:        assert.Error,
		},
	}

	for _, test := range tests {
		suite.Run(test.name, func() {
			t := suite.T()

			ctx, flush := tester.NewContext(t)
			defer flush()

			called := 0
			mw := newTestMW(
				func(*http.Request) { called++ },
				newMWReturns(test.status, nil, test.providedErr))
			mw.repeatReturn0 = true

			// Add a large timeout of 100 seconds to ensure that the ctx deadline
			// doesn't exceed. Otherwise, we'll end up retrying due to ctx deadline
			// exceeded, instead of the actual test case. This is also important
			// for 503 and 504 test cases which are handled by kiota retry handler.
			// We don't want corso retry handler to kick in for these cases.
			cc := populateConfig(
				MinimumBackoff(10*time.Millisecond),
				Timeout(100*time.Second),
				MaxRetries(test.expectRetryCount))

			adpt, err := mockAdapter(suite.creds, mw, cc)
			require.NoError(t, err, clues.ToCore(err))

			// url doesn't fit the builder, but that shouldn't matter
			_, err = users.NewCountRequestBuilder(url, adpt).Get(ctx, nil)
			test.expectErr(t, err, clues.ToCore(err))

			// -1 because the non-retried call always counts for one, then
			// we increment based on the number of retry attempts.
			assert.Equal(t, test.expectRetryCount, called-1)
		})
	}
}

func (suite *RetryMWIntgSuite) TestRetryMiddleware_RetryRequest_resetBodyAfter500() {
	t := suite.T()

	ctx, flush := tester.NewContext(t)
	defer flush()

	var (
		body             = models.NewMailFolder()
		checkOnIntercept = func(req *http.Request) {
			bs, err := io.ReadAll(req.Body)
			require.NoError(t, err, clues.ToCore(err))

			// an expired body, after graph compression, will
			// normally contain 25 bytes.  So we should see more
			// than that at least.
			require.Less(
				t,
				25,
				len(bs),
				"body should be longer than 25 bytes; shorter indicates the body was sliced on a retry")
		}
	)

	body.SetDisplayName(ptr.To(uuid.NewString()))

	mw := newTestMW(
		checkOnIntercept,
		newMWReturns(http.StatusInternalServerError, nil, nil),
		newMWReturns(http.StatusOK, nil, nil))

	cc := populateConfig(
		MinimumBackoff(10*time.Millisecond),
		Timeout(100*time.Second))

	adpt, err := mockAdapter(suite.creds, mw, cc)
	require.NoError(t, err, clues.ToCore(err))

	// no api package needed here, this is a mocked request that works
	// independent of the query.
	_, err = NewService(adpt).
		Client().
		Users().
		ByUserId("user").
		MailFolders().
		Post(ctx, body, nil)
	require.NoError(t, err, clues.ToCore(err))
}

func (suite *RetryMWIntgSuite) TestRetryMiddleware_RetryResponse_maintainBodyAfter503() {
	t := suite.T()

	ctx, flush := tester.NewContext(t)
	defer flush()

	odem := graphTD.ODataErrWithMsg("SystemDown", "The System, Is Down, bah-dup-da-woo-woo!")
	m := graphTD.ParseableToMap(t, odem)

	body, err := json.Marshal(m)
	require.NoError(t, err, clues.ToCore(err))

	mw := newTestMW(
		// intentional no-op, just need to conrol the response code
		func(*http.Request) {},
		newMWReturns(http.StatusServiceUnavailable, body, nil),
		newMWReturns(http.StatusServiceUnavailable, body, nil))

	// Configure max retries to 1 so that the test case doesn't run for too
	// long due to exponential backoffs. Also, add a large timeout of 100 seconds
	// to ensure that the ctx deadline doesn't exceed. Otherwise, we'll end up
	// retrying due to timeout exceeded, instead of 503s.
	cc := populateConfig(
		MaxRetries(1),
		MinimumBackoff(1*time.Second),
		Timeout(100*time.Second))

	adpt, err := mockAdapter(suite.creds, mw, cc)
	require.NoError(t, err, clues.ToCore(err))

	// no api package needed here,
	// this is a mocked request that works
	// independent of the query.
	_, err = NewService(adpt).
		Client().
		Users().
		ByUserId("user").
		MailFolders().
		Post(ctx, models.NewMailFolder(), nil)
	require.Error(t, err, clues.ToCore(err))
	require.NotContains(t, err.Error(), "content is empty", clues.ToCore(err))
	require.Contains(t, err.Error(), "503", clues.ToCore(err))
}

type MiddlewareUnitSuite struct {
	tester.Suite
}

func TestMiddlewareUnitSuite(t *testing.T) {
	suite.Run(t, &MiddlewareUnitSuite{Suite: tester.NewUnitSuite(t)})
}

func (suite *MiddlewareUnitSuite) TestBindExtractLimiterConfig() {
	t := suite.T()

	ctx, flush := tester.NewContext(t)
	defer flush()

	// an unpopulated ctx should produce the default limiter
	assert.Equal(t, defaultLimiter, ctxLimiter(ctx))

	table := []struct {
		name             string
		service          path.ServiceType
		enableSlidingLim bool
		expectLimiter    limiters.Limiter
	}{
		{
			name:          "exchange",
			service:       path.ExchangeService,
			expectLimiter: defaultLimiter,
		},
		{
			name:          "oneDrive",
			service:       path.OneDriveService,
			expectLimiter: driveLimiter,
		},
		{
			name:          "sharePoint",
			service:       path.SharePointService,
			expectLimiter: driveLimiter,
		},
		{
			name:          "groups",
			service:       path.GroupsService,
			expectLimiter: driveLimiter,
		},
		{
			name:          "unknownService",
			service:       path.UnknownService,
			expectLimiter: defaultLimiter,
		},
		{
			name:          "badService",
			service:       path.ServiceType(-1),
			expectLimiter: defaultLimiter,
		},
		{
			name:             "exchange sliding limiter",
			service:          path.ExchangeService,
			enableSlidingLim: true,
			expectLimiter:    exchSlidingLimiter,
		},
		// Sliding limiter flag is ignored for non-exchange services
		{
			name:             "onedrive with sliding limiter flag set",
			service:          path.OneDriveService,
			enableSlidingLim: true,
			expectLimiter:    driveLimiter,
		},
	}
	for _, test := range table {
		suite.Run(test.name, func() {
			t := suite.T()

			tctx := BindRateLimiterConfig(
				ctx,
				LimiterCfg{
					Service:              test.service,
					EnableSlidingLimiter: test.enableSlidingLim,
				})
			lc, ok := extractRateLimiterConfig(tctx)
			require.True(t, ok, "found rate limiter in ctx")
			assert.Equal(t, test.service, lc.Service)
			assert.Equal(t, test.expectLimiter, ctxLimiter(tctx))
		})
	}
}

func (suite *MiddlewareUnitSuite) TestLimiterConsumption() {
	t := suite.T()

	ctx, flush := tester.NewContext(t)
	defer flush()

	// an unpopulated ctx should produce the default consumption
	assert.Equal(t, defaultLC, ctxLimiterConsumption(ctx, defaultLC))

	table := []struct {
		name   string
		n      int
		expect int
	}{
		{
			name:   "matches default",
			n:      defaultLC,
			expect: defaultLC,
		},
		{
			name:   "default+1",
			n:      defaultLC + 1,
			expect: defaultLC + 1,
		},
		{
			name:   "zero",
			n:      0,
			expect: defaultLC,
		},
		{
			name:   "negative",
			n:      -1,
			expect: defaultLC,
		},
	}
	for _, test := range table {
		suite.Run(test.name, func() {
			t := suite.T()

			tctx := ConsumeNTokens(ctx, test.n)
			lc := ctxLimiterConsumption(tctx, defaultLC)
			assert.Equal(t, test.expect, lc)
		})
	}
}

const (
	// Raw test token valid for 100 years.
	rawToken = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9." +
		"eyJuYmYiOiIxNjkxODE5NTc5IiwiZXhwIjoiMzk0NTUyOTE3OSIsImVuZHBvaW50dXJsTGVuZ3RoIjoiMTYw" +
		"IiwiaXNsb29wYmFjayI6IlRydWUiLCJ2ZXIiOiJoYXNoZWRwcm9vZnRva2VuIiwicm9sZXMiOiJhbGxmaWxl" +
		"cy53cml0ZSBhbGxzaXRlcy5mdWxsY29udHJvbCBhbGxwcm9maWxlcy5yZWFkIiwidHQiOiIxIiwiYWxnIjoi" +
		"SFMyNTYifQ" +
		".signature"
)

// Tests getTokenLifetime
func (suite *MiddlewareUnitSuite) TestGetTokenLifetime() {
	table := []struct {
		name      string
		request   *http.Request
		expectErr assert.ErrorAssertionFunc
	}{
		{
			name:      "nil request",
			request:   nil,
			expectErr: assert.Error,
		},
		// Test that we don't throw an error if auth header is absent.
		// This is to prevent unnecessary noise in logs for requestor http client.
		{
			name: "no authorization header",
			request: &http.Request{
				Header: http.Header{},
			},
			expectErr: assert.NoError,
		},
		{
			name: "well formed auth header with token",
			request: &http.Request{
				Header: http.Header{
					"Authorization": []string{"Bearer " + rawToken},
				},
			},
			expectErr: assert.NoError,
		},
		{
			name: "Missing Bearer prefix but valid token",
			request: &http.Request{
				Header: http.Header{
					"Authorization": []string{rawToken},
				},
			},
			expectErr: assert.NoError,
		},
		{
			name: "invalid token",
			request: &http.Request{
				Header: http.Header{
					"Authorization": []string{"Bearer " + "invalid"},
				},
			},
			expectErr: assert.Error,
		},
		{
			name: "valid prefix but empty token",
			request: &http.Request{
				Header: http.Header{
					"Authorization": []string{"Bearer "},
				},
			},
			expectErr: assert.Error,
		},
		{
			name: "Invalid prefix but valid token",
			request: &http.Request{
				Header: http.Header{
					"Authorization": []string{"Bearer" + rawToken},
				},
			},
			expectErr: assert.Error,
		},
	}

	for _, test := range table {
		suite.Run(test.name, func() {
			t := suite.T()

			ctx, flush := tester.NewContext(t)
			defer flush()

			// iat, exp specific tests are in jwt package.
			_, _, err := getTokenLifetime(ctx, test.request)
			test.expectErr(t, err, clues.ToCore(err))
		})
	}
}
