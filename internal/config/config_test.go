package config

import "testing"

func TestNormalizeInterfacePolicy(t *testing.T) {
	cases := map[string]string{
		"auto":     "auto",
		" manual ": "manual",
		"ALL":      "all",
		"":         "auto",
		"weird":    "auto",
	}
	for in, want := range cases {
		if got := NormalizeInterfacePolicy(in); got != want {
			t.Fatalf("normalize %q: got %q want %q", in, got, want)
		}
	}
}

func TestNormalizeDownloadPipelineMode(t *testing.T) {
	cases := map[string]string{
		"legacy":    "legacy",
		" LEGACY ":  "legacy",
		"pipelined": "pipelined",
		"weird":     "legacy",
		"":          "legacy",
	}
	for in, want := range cases {
		if got := NormalizeDownloadPipelineMode(in); got != want {
			t.Fatalf("normalize pipeline %q: got %q want %q", in, got, want)
		}
	}
}

func TestNormalizeDownloadChannelStrategy(t *testing.T) {
	cases := map[string]string{
		"dynamic":       "dynamic",
		" strict_split": "strict_split",
		"STRICT_SPLIT":  "strict_split",
		"":              "dynamic",
		"weird":         "dynamic",
	}
	for in, want := range cases {
		if got := NormalizeDownloadChannelStrategy(in); got != want {
			t.Fatalf("normalize strategy %q: got %q want %q", in, got, want)
		}
	}
}

func TestNormalizePullFolderResult(t *testing.T) {
	cases := map[string]string{
		"zip":       "zip",
		" ZIP ":     "zip",
		"extract":   "extract",
		"EXTRACT":   "extract",
		"":          "zip",
		"something": "zip",
	}
	for in, want := range cases {
		if got := NormalizePullFolderResult(in); got != want {
			t.Fatalf("normalize pull folder result %q: got %q want %q", in, got, want)
		}
	}
}
