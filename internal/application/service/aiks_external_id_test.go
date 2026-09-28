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

func TestManualContentLimitKeepsAIKSSeparateFromNormalManualInput(t *testing.T) {
	if got := manualContentLimit(types.ChannelWeb); got != manualContentMaxLength {
		t.Fatalf("web limit = %d, want %d", got, manualContentMaxLength)
	}
	if got := manualContentLimit(types.ChannelAIKS); got != aiksManualContentMaxLength {
		t.Fatalf("AIKS limit = %d, want %d", got, aiksManualContentMaxLength)
	}
	if aiksManualContentMaxLength <= manualContentMaxLength {
		t.Fatal("AIKS limit must be larger than the interactive manual limit")
	}
}

func TestSameAIKSManualReplayRequiresHealthyEquivalentDocument(t *testing.T) {
	meta := types.NewManualKnowledgeMetadata("# body", types.ManualKnowledgeStatusPublish, 1)
	knowledge := &types.Knowledge{
		Title:       "Session",
		Channel:     types.ChannelAIKS,
		ParseStatus: "completed",
	}
	if err := knowledge.SetManualMetadata(meta); err != nil {
		t.Fatal(err)
	}
	if !sameAIKSManualReplay(knowledge, "# body", types.ManualKnowledgeStatusPublish, "Session") {
		t.Fatal("equivalent healthy replay was not recognized")
	}
	knowledge.ParseStatus = "failed"
	if sameAIKSManualReplay(knowledge, "# body", types.ManualKnowledgeStatusPublish, "Session") {
		t.Fatal("failed document must be reprocessed instead of accepted as an idempotent replay")
	}
	knowledge.ParseStatus = "completed"
	if sameAIKSManualReplay(knowledge, "# changed", types.ManualKnowledgeStatusPublish, "Session") {
		t.Fatal("changed content must update the existing knowledge")
	}
}
