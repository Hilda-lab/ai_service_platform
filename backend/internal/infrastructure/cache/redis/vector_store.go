package redis

import (
	"context"
	"fmt"
	"strconv"

	goredis "github.com/redis/go-redis/v9"
)

type VectorStore struct {
	client *goredis.Client
}

func NewVectorStore(client *goredis.Client) *VectorStore {
	return &VectorStore{client: client}
}

// 存储向量元数据和向量数据
// Key 结构: rag:user:{userId}:chunk:{chunkId} -> {"document_id": X, "content": "...", "vector": [...]}
// 索引: rag:user:{userId}:doc:{docId}:chunks -> [chunkId1, chunkId2, ...]
//      rag:user:{userId}:chunks -> [chunkId1, chunkId2, ...]

// StoreChunk 存储单个分块的向量和元数据
func (vs *VectorStore) StoreChunk(ctx context.Context, userID uint, chunkID uint, documentID uint, content string, vector []float64) error {
	// JSON 格式存储向量数据
	vectorJSON := ""
	if len(vector) > 0 {
		// 序列化向量（逗号分隔的浮点数字符串，以节省空间）
		vectorBytes := make([]byte, 0)
		for i, v := range vector {
			if i > 0 {
				vectorBytes = append(vectorBytes, ',')
			}
			vectorBytes = append(vectorBytes, []byte(fmt.Sprintf("%.6f", v))...)
		}
		vectorJSON = string(vectorBytes)
	}

	key := fmt.Sprintf("rag:user:%d:chunk:%d", userID, chunkID)
	
	// 使用 Hash 结构存储元数据
	err := vs.client.HSet(ctx, key, map[string]interface{}{
		"document_id": documentID,
		"content":     content,
		"vector":      vectorJSON,
	}).Err()
	
	if err != nil {
		return err
	}

	// 添加索引
	// 1. 分块ID到用户的全局索引
	userChunksKey := fmt.Sprintf("rag:user:%d:chunks", userID)
	if err := vs.client.SAdd(ctx, userChunksKey, chunkID).Err(); err != nil {
		return err
	}

	// 2. 分块ID到文档的索引
	docChunksKey := fmt.Sprintf("rag:user:%d:doc:%d:chunks", userID, documentID)
	if err := vs.client.SAdd(ctx, docChunksKey, chunkID).Err(); err != nil {
		return err
	}

	return nil
}

// GetChunk 获取单个分块数据
func (vs *VectorStore) GetChunk(ctx context.Context, userID uint, chunkID uint) (map[string]string, error) {
	key := fmt.Sprintf("rag:user:%d:chunk:%d", userID, chunkID)
	result, err := vs.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("chunk not found")
	}
	return result, nil
}

// ListUserChunks 获取用户的所有分块ID
func (vs *VectorStore) ListUserChunks(ctx context.Context, userID uint) ([]uint, error) {
	userChunksKey := fmt.Sprintf("rag:user:%d:chunks", userID)
	members, err := vs.client.SMembers(ctx, userChunksKey).Result()
	if err != nil {
		return nil, err
	}

	chunkIDs := make([]uint, 0, len(members))
	for _, member := range members {
		id, err := strconv.ParseUint(member, 10, 32)
		if err != nil {
			continue
		}
		chunkIDs = append(chunkIDs, uint(id))
	}
	return chunkIDs, nil
}

// ListDocumentChunks 获取文档的所有分块ID
func (vs *VectorStore) ListDocumentChunks(ctx context.Context, userID uint, documentID uint) ([]uint, error) {
	docChunksKey := fmt.Sprintf("rag:user:%d:doc:%d:chunks", userID, documentID)
	members, err := vs.client.SMembers(ctx, docChunksKey).Result()
	if err != nil {
		return nil, err
	}

	chunkIDs := make([]uint, 0, len(members))
	for _, member := range members {
		id, err := strconv.ParseUint(member, 10, 32)
		if err != nil {
			continue
		}
		chunkIDs = append(chunkIDs, uint(id))
	}
	return chunkIDs, nil
}

// DeleteDocument 删除文档的所有向量数据
func (vs *VectorStore) DeleteDocument(ctx context.Context, userID uint, documentID uint) error {
	// 获取所有分块
	docChunksKey := fmt.Sprintf("rag:user:%d:doc:%d:chunks", userID, documentID)
	chunkIDs, err := vs.ListDocumentChunks(ctx, userID, documentID)
	if err != nil {
		return err
	}

	// 删除每个分块
	for _, chunkID := range chunkIDs {
		key := fmt.Sprintf("rag:user:%d:chunk:%d", userID, chunkID)
		if err := vs.client.Del(ctx, key).Err(); err != nil {
			return err
		}
		// 从用户全局索引中删除
		userChunksKey := fmt.Sprintf("rag:user:%d:chunks", userID)
		if err := vs.client.SRem(ctx, userChunksKey, chunkID).Err(); err != nil {
			return err
		}
	}

	// 删除文档索引
	if err := vs.client.Del(ctx, docChunksKey).Err(); err != nil {
		return err
	}

	return nil
}

// DeleteChunk 删除单个分块
func (vs *VectorStore) DeleteChunk(ctx context.Context, userID uint, chunkID uint, documentID uint) error {
	key := fmt.Sprintf("rag:user:%d:chunk:%d", userID, chunkID)
	userChunksKey := fmt.Sprintf("rag:user:%d:chunks", userID)
	docChunksKey := fmt.Sprintf("rag:user:%d:doc:%d:chunks", userID, documentID)

	if err := vs.client.Del(ctx, key).Err(); err != nil {
		return err
	}
	if err := vs.client.SRem(ctx, userChunksKey, chunkID).Err(); err != nil {
		return err
	}
	if err := vs.client.SRem(ctx, docChunksKey, chunkID).Err(); err != nil {
		return err
	}

	return nil
}
