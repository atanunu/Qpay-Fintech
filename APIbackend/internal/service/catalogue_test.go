package service

import (
	"os"
	"testing"
)

func TestEmbeddedCatalogueExactlyMatchesNovu(t *testing.T) {
	canonical, e := os.ReadFile("../../../Novu/catalogue/emails.psv")
	if e != nil {
		t.Fatal(e)
	}
	if string(canonical) != notificationCatalogue {
		t.Fatal("embedded catalogue drift; regenerate from canonical Novu source")
	}
	if len(NotificationMetadata) != 164 {
		t.Fatal("unexpected catalogue size")
	}
	for _, key := range []string{"transfer-completed", "funding-received", "identity-verify-email", "support-reply-received"} {
		if _, exists := NotificationMetadata[key]; !exists {
			t.Fatal("missing workflow", key)
		}
	}
}
