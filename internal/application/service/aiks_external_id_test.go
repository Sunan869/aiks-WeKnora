package service

import (
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestValidateManualExternalID(t *testing.T) {
	valid := "aiks-" + strings.Repeat("a", 64)
	got, err := validateManualExternalID(types.ChannelAIKS, valid)
	if err != nil || got != valid {
		t.Fatalf("valid AIKS external id rejected: got=%q err=%v", got, err)
	}
	if _, err := validateManualExternalID(types.ChannelAPI, valid); err == nil {
		t.Fatal("non-AIKS channel accepted external id")
	}
	if _, err := validateManualExternalID(types.ChannelAIKS, "aiks-short"); err == nil {
		t.Fatal("short external id accepted")
	}
	if _, err := validateManualExternalID(types.ChannelAIKS, "aiks-"+strings.Repeat("Z", 64)); err == nil {
		t.Fatal("non-hex external id accepted")
	}
}

func TestManualMetadataPreservesAIKSExternalID(t *testing.T) {
	valid := "aiks-" + strings.Repeat("b", 64)
	meta := types.NewManualKnowledgeMetadata("# body", types.ManualKnowledgeStatusPublish, 1)
	meta.ExternalID = valid
	knowledge := &types.Knowledge{}
	if err := knowledge.SetManualMetadata(meta); err != nil {
		t.Fatal(err)
	}
	loaded, err := knowledge.ManualMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.ExternalID != valid {
		t.Fatalf("external id did not round-trip: %#v", loaded)
	}
}
