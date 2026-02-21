package rag

import (
	"context"

	config_types "xiaozhi-esp32-server-golang/internal/domain/config/types"
)

// Searcher 按 provider 实现知识库检索。
type Searcher interface {
	Search(
		ctx context.Context,
		query string,
		topK int,
		knowledgeBases []config_types.KnowledgeBaseRef,
		providerConfig map[string]interface{},
	) ([]config_types.KnowledgeSearchHit, error)
}
