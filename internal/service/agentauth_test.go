package service

import "testing"

func TestAgentAuthHashTokenIsStableAndDoesNotReturnPlaintext(t *testing.T) {
	token := "refresh-token-value"

	first := HashAgentToken(token)
	second := HashAgentToken(token)

	if first == "" {
		t.Fatal("expected token hash")
	}
	if first != second {
		t.Fatalf("expected stable hash, got %q and %q", first, second)
	}
	if first == token {
		t.Fatal("hash must not equal plaintext token")
	}
}

func TestAgentAuthRefreshRotationRejectsReusedToken(t *testing.T) {
	oldToken := "old-refresh-token"
	device := AgentDeviceSession{
		RefreshTokenHash: HashAgentToken(oldToken),
	}

	newToken, err := RotateAgentRefreshToken(&device, oldToken)
	if err != nil {
		t.Fatalf("expected first rotation to succeed: %v", err)
	}
	if newToken == "" {
		t.Fatal("expected new refresh token")
	}
	if device.RefreshTokenHash != HashAgentToken(newToken) {
		t.Fatal("expected session hash to be updated to new token")
	}

	if _, err := RotateAgentRefreshToken(&device, oldToken); err == nil {
		t.Fatal("expected reused refresh token to be rejected")
	}
}
