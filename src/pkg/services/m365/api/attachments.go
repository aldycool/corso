package api

import (
	"strings"

	"github.com/microsoftgraph/msgraph-sdk-go/models"

	"github.com/alcionai/corso/src/internal/common/ptr"
)

func HasAttachments(body models.ItemBodyable) bool {
	if body == nil {
		return false
	}

	if ct, ok := ptr.ValOK(body.GetContentType()); !ok || ct == models.TEXT_BODYTYPE {
		return false
	}

	if body, ok := ptr.ValOK(body.GetContent()); !ok || len(body) == 0 {
		return false
	}

	return strings.Contains(ptr.Val(body.GetContent()), "src=\"cid:")
}