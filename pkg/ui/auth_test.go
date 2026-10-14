package ui

import "testing"

func TestResolveSignIn(t *testing.T) {
	tests := []struct {
		name                           string
		isAuthenticating, needPassword bool
		username                       string
		want                           signInIntent
	}{
		{"idle with username authenticates", false, false, "alice", signInAuthenticate},
		{"idle without username does nothing", false, false, "", signInNone},
		{"awaiting password submits response", true, true, "alice", signInRespond},
		{"authenticating without prompt does nothing", true, false, "alice", signInNone},
		{"idle but prompting still authenticates with a username", false, true, "alice", signInAuthenticate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveSignIn(tt.isAuthenticating, tt.needPassword, tt.username)
			if got != tt.want {
				t.Fatalf("resolveSignIn(authenticating=%v, needPassword=%v, username=%q) = %v, want %v",
					tt.isAuthenticating, tt.needPassword, tt.username, got, tt.want)
			}
		})
	}
}
