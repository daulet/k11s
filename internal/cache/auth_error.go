package cache

import (
	"fmt"
	"strings"
)

func FriendlyKubeAccessError(err error, kubeContext string) string {
	if err == nil {
		return ""
	}
	raw := strings.TrimSpace(err.Error())
	if raw == "" {
		return "cluster access error"
	}

	lower := strings.ToLower(raw)
	if !looksLikeAuthExpiry(lower) {
		return raw
	}

	contextLabel := strings.TrimSpace(kubeContext)
	message := "cluster authentication expired"
	if contextLabel != "" {
		message = fmt.Sprintf("cluster authentication expired for context %s", contextLabel)
	}

	if cmd := reloginHint(lower); cmd != "" {
		return fmt.Sprintf("%s; run `%s` and retry", message, cmd)
	}
	return message + "; re-authenticate and retry"
}

func looksLikeAuthExpiry(lower string) bool {
	switch {
	case strings.Contains(lower, "getting credentials"):
		return true
	case strings.Contains(lower, "provide credentials"):
		return true
	case strings.Contains(lower, "unauthorized"):
		return true
	case strings.Contains(lower, "token has expired"):
		return true
	case strings.Contains(lower, "expired token"):
		return true
	case strings.Contains(lower, "credentials are expired"):
		return true
	case strings.Contains(lower, "exec: executable") && strings.Contains(lower, "failed with exit code"):
		return true
	default:
		return false
	}
}

func reloginHint(lower string) string {
	switch {
	case strings.Contains(lower, "tsh"):
		return "tsh login"
	case strings.Contains(lower, "aws") && strings.Contains(lower, "sso"):
		return "aws sso login"
	case strings.Contains(lower, "gcloud"):
		return "gcloud auth login"
	case strings.Contains(lower, "azure") || strings.Contains(lower, "az "):
		return "az login"
	default:
		return ""
	}
}
