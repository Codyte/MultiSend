package config

import "testing"

func TestGenerateSecretUniqueAndNonEmpty(t *testing.T) {
	a := GenerateSecret()
	b := GenerateSecret()
	if len(a) != 64 || len(b) != 64 {
		t.Fatalf("secret must contain 256 bits encoded as hex: len(a)=%d len(b)=%d", len(a), len(b))
	}
	if a == b {
		t.Fatal("secrets must be unique")
	}
}

func TestAuthSecretGatedByRequireAuth(t *testing.T) {
	c := Config{NodeSecret: GenerateSecret()}
	if c.AuthSecret() != nil {
		t.Fatal("AuthSecret must be nil when RequireAuth is false")
	}
	c.RequireAuth = true
	if c.AuthSecret() == nil {
		t.Fatal("AuthSecret must be set when RequireAuth is true")
	}
}

func TestSecretBytesDecodesHex(t *testing.T) {
	c := Config{NodeSecret: "00ff10"}
	b := c.SecretBytes()
	if len(b) != 3 || b[0] != 0x00 || b[1] != 0xff || b[2] != 0x10 {
		t.Fatalf("unexpected decoded secret: %v", b)
	}
}

func TestDefaultConfigSeedsSecurityFields(t *testing.T) {
	c := DefaultConfig()
	if c.NodeSecret == "" {
		t.Fatal("default config must seed a node secret")
	}
	if c.RemoteSendRoots == nil {
		t.Fatal("RemoteSendRoots must be initialized")
	}
	if c.SchemaVersion < 4 {
		t.Fatalf("schema version must be >= 4, got %d", c.SchemaVersion)
	}
}

func TestMigrateGeneratesSecret(t *testing.T) {
	c := &Config{SchemaVersion: 3}
	if !migrateLegacyConfig(c) {
		t.Fatal("migration should report changes")
	}
	if c.NodeSecret == "" {
		t.Fatal("migration must backfill a node secret")
	}
	if c.SchemaVersion < 4 {
		t.Fatalf("migration must bump schema to >= 4, got %d", c.SchemaVersion)
	}
}
