package cmd

import "testing"

func TestParseGitRemote(t *testing.T) {
	tests := []struct {
		url       string
		wantOwner string
		wantRepo  string
	}{
		{"https://github.com/privsim/fleet-deploy.git", "privsim", "fleet-deploy"},
		{"https://github.com/privsim/fleet-deploy", "privsim", "fleet-deploy"},
		{"git@github.com:disentangle-network/fleet.git", "disentangle-network", "fleet"},
		{"git@github.com:disentangle-network/fleet", "disentangle-network", "fleet"},
		{"https://github.com/LarsenClose/genesis-operator.git", "LarsenClose", "genesis-operator"},
		{"", "", ""},
	}

	for _, tt := range tests {
		owner, repo := parseGitRemote(tt.url)
		if owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("parseGitRemote(%q) = (%q, %q), want (%q, %q)",
				tt.url, owner, repo, tt.wantOwner, tt.wantRepo)
		}
	}
}
