package handler

import "testing"

func TestRejectPrivateHost_Loopback_Rejected(t *testing.T) {
	if err := rejectPrivateHost("localhost"); err == nil {
		t.Error("expected localhost to be rejected")
	}
}

func TestRejectPrivateHost_PrivateIP_Rejected(t *testing.T) {
	if err := rejectPrivateHost("192.168.1.1"); err == nil {
		t.Error("expected a private IP to be rejected")
	}
}

func TestRejectPrivateHost_LinkLocal_Rejected(t *testing.T) {
	if err := rejectPrivateHost("169.254.169.254"); err == nil {
		t.Error("expected a link-local address (cloud metadata endpoint) to be rejected")
	}
}

func TestRejectPrivateHost_PublicHost_Allowed(t *testing.T) {
	if err := rejectPrivateHost("open.live.bbc.co.uk"); err != nil {
		t.Errorf("expected a real public host to be allowed, got: %v", err)
	}
}
