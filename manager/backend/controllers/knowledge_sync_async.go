package controllers

import (
	"fmt"
	"log"
	"sync"
	"time"

	"xiaozhi/manager/backend/models"

	"gorm.io/gorm"
)

const (
	knowledgeSyncQueueSize   = 256
	knowledgeSyncWorkerCount = 2
)

type knowledgeSyncJobType string

const (
	knowledgeSyncJobUpsert    knowledgeSyncJobType = "upsert"
	knowledgeSyncJobDelete    knowledgeSyncJobType = "delete"
	knowledgeSyncJobDocUpsert knowledgeSyncJobType = "doc_upsert"
	knowledgeSyncJobDocDelete knowledgeSyncJobType = "doc_delete"
)

type knowledgeSyncJob struct {
	jobType           knowledgeSyncJobType
	db                *gorm.DB
	knowledgeBaseID   uint
	documentID        uint
	knowledgeSnapshot *models.KnowledgeBase
	documentSnapshot  *models.KnowledgeBaseDocument
	enqueuedAt        time.Time
}

var (
	knowledgeSyncQueue     chan knowledgeSyncJob
	knowledgeSyncQueueOnce sync.Once
)

func ensureKnowledgeSyncWorkersStarted() {
	knowledgeSyncQueueOnce.Do(func() {
		knowledgeSyncQueue = make(chan knowledgeSyncJob, knowledgeSyncQueueSize)
		for i := 1; i <= knowledgeSyncWorkerCount; i++ {
			go runKnowledgeSyncWorker(i)
		}
		log.Printf("[KnowledgeSync][Async] workers started count=%d queue_size=%d", knowledgeSyncWorkerCount, knowledgeSyncQueueSize)
	})
}

func enqueueKnowledgeSyncUpsert(db *gorm.DB, knowledgeBaseID uint) error {
	if db == nil {
		return fmt.Errorf("数据库连接为空")
	}
	if knowledgeBaseID == 0 {
		return fmt.Errorf("无效的知识库ID")
	}
	ensureKnowledgeSyncWorkersStarted()

	job := knowledgeSyncJob{
		jobType:         knowledgeSyncJobUpsert,
		db:              db,
		knowledgeBaseID: knowledgeBaseID,
		enqueuedAt:      time.Now(),
	}
	select {
	case knowledgeSyncQueue <- job:
		log.Printf("[KnowledgeSync][Async] enqueue type=%s kb_id=%d", job.jobType, job.knowledgeBaseID)
		return nil
	default:
		return fmt.Errorf("知识库同步队列已满，请稍后重试")
	}
}

func enqueueKnowledgeSyncDelete(db *gorm.DB, snapshot models.KnowledgeBase) error {
	if db == nil {
		return fmt.Errorf("数据库连接为空")
	}
	ensureKnowledgeSyncWorkersStarted()

	s := snapshot
	job := knowledgeSyncJob{
		jobType:           knowledgeSyncJobDelete,
		db:                db,
		knowledgeBaseID:   snapshot.ID,
		knowledgeSnapshot: &s,
		enqueuedAt:        time.Now(),
	}
	select {
	case knowledgeSyncQueue <- job:
		log.Printf("[KnowledgeSync][Async] enqueue type=%s kb_id=%d", job.jobType, job.knowledgeBaseID)
		return nil
	default:
		return fmt.Errorf("知识库同步队列已满，请稍后重试")
	}
}

func enqueueKnowledgeDocumentSyncUpsert(db *gorm.DB, knowledgeBaseID, documentID uint) error {
	if db == nil {
		return fmt.Errorf("数据库连接为空")
	}
	if knowledgeBaseID == 0 || documentID == 0 {
		return fmt.Errorf("无效的知识库或文档ID")
	}
	ensureKnowledgeSyncWorkersStarted()

	job := knowledgeSyncJob{
		jobType:         knowledgeSyncJobDocUpsert,
		db:              db,
		knowledgeBaseID: knowledgeBaseID,
		documentID:      documentID,
		enqueuedAt:      time.Now(),
	}
	select {
	case knowledgeSyncQueue <- job:
		log.Printf("[KnowledgeSync][Async] enqueue type=%s kb_id=%d doc_id=%d", job.jobType, job.knowledgeBaseID, job.documentID)
		return nil
	default:
		return fmt.Errorf("知识库同步队列已满，请稍后重试")
	}
}

func enqueueKnowledgeDocumentSyncDelete(db *gorm.DB, kbSnapshot models.KnowledgeBase, docSnapshot models.KnowledgeBaseDocument) error {
	if db == nil {
		return fmt.Errorf("数据库连接为空")
	}
	ensureKnowledgeSyncWorkersStarted()

	kb := kbSnapshot
	doc := docSnapshot
	job := knowledgeSyncJob{
		jobType:           knowledgeSyncJobDocDelete,
		db:                db,
		knowledgeBaseID:   kbSnapshot.ID,
		documentID:        docSnapshot.ID,
		knowledgeSnapshot: &kb,
		documentSnapshot:  &doc,
		enqueuedAt:        time.Now(),
	}
	select {
	case knowledgeSyncQueue <- job:
		log.Printf("[KnowledgeSync][Async] enqueue type=%s kb_id=%d doc_id=%d", job.jobType, job.knowledgeBaseID, job.documentID)
		return nil
	default:
		return fmt.Errorf("知识库同步队列已满，请稍后重试")
	}
}

func runKnowledgeSyncWorker(workerID int) {
	for job := range knowledgeSyncQueue {
		waitMs := time.Since(job.enqueuedAt).Milliseconds()
		start := time.Now()
		switch job.jobType {
		case knowledgeSyncJobUpsert:
			err := processKnowledgeSyncUpsert(job)
			if err != nil {
				log.Printf("[KnowledgeSync][Async] worker=%d type=%s kb_id=%d wait_ms=%d cost_ms=%d err=%v", workerID, job.jobType, job.knowledgeBaseID, waitMs, time.Since(start).Milliseconds(), err)
			} else {
				log.Printf("[KnowledgeSync][Async] worker=%d type=%s kb_id=%d wait_ms=%d cost_ms=%d status=ok", workerID, job.jobType, job.knowledgeBaseID, waitMs, time.Since(start).Milliseconds())
			}
		case knowledgeSyncJobDelete:
			err := processKnowledgeSyncDelete(job)
			if err != nil {
				log.Printf("[KnowledgeSync][Async] worker=%d type=%s kb_id=%d wait_ms=%d cost_ms=%d err=%v", workerID, job.jobType, job.knowledgeBaseID, waitMs, time.Since(start).Milliseconds(), err)
			} else {
				log.Printf("[KnowledgeSync][Async] worker=%d type=%s kb_id=%d wait_ms=%d cost_ms=%d status=ok", workerID, job.jobType, job.knowledgeBaseID, waitMs, time.Since(start).Milliseconds())
			}
		case knowledgeSyncJobDocUpsert:
			err := processKnowledgeDocumentSyncUpsert(job)
			if err != nil {
				log.Printf("[KnowledgeSync][Async] worker=%d type=%s kb_id=%d doc_id=%d wait_ms=%d cost_ms=%d err=%v", workerID, job.jobType, job.knowledgeBaseID, job.documentID, waitMs, time.Since(start).Milliseconds(), err)
			} else {
				log.Printf("[KnowledgeSync][Async] worker=%d type=%s kb_id=%d doc_id=%d wait_ms=%d cost_ms=%d status=ok", workerID, job.jobType, job.knowledgeBaseID, job.documentID, waitMs, time.Since(start).Milliseconds())
			}
		case knowledgeSyncJobDocDelete:
			err := processKnowledgeDocumentSyncDelete(job)
			if err != nil {
				log.Printf("[KnowledgeSync][Async] worker=%d type=%s kb_id=%d doc_id=%d wait_ms=%d cost_ms=%d err=%v", workerID, job.jobType, job.knowledgeBaseID, job.documentID, waitMs, time.Since(start).Milliseconds(), err)
			} else {
				log.Printf("[KnowledgeSync][Async] worker=%d type=%s kb_id=%d doc_id=%d wait_ms=%d cost_ms=%d status=ok", workerID, job.jobType, job.knowledgeBaseID, job.documentID, waitMs, time.Since(start).Milliseconds())
			}
		default:
			log.Printf("[KnowledgeSync][Async] worker=%d unknown_job_type=%s kb_id=%d", workerID, job.jobType, job.knowledgeBaseID)
		}
	}
}

func processKnowledgeSyncUpsert(job knowledgeSyncJob) error {
	if job.db == nil {
		return fmt.Errorf("数据库连接为空")
	}
	var kb models.KnowledgeBase
	if err := job.db.Where("id = ?", job.knowledgeBaseID).First(&kb).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return fmt.Errorf("加载知识库失败: %w", err)
	}
	return syncKnowledgeBaseBestEffort(job.db, &kb)
}

func processKnowledgeSyncDelete(job knowledgeSyncJob) error {
	if job.db == nil {
		return fmt.Errorf("数据库连接为空")
	}
	if job.knowledgeSnapshot == nil {
		return fmt.Errorf("删除同步缺少知识库快照")
	}
	return syncKnowledgeBaseDeleteBestEffort(job.db, job.knowledgeSnapshot)
}

func processKnowledgeDocumentSyncUpsert(job knowledgeSyncJob) error {
	if job.db == nil {
		return fmt.Errorf("数据库连接为空")
	}
	return syncKnowledgeDocumentBestEffort(job.db, job.knowledgeBaseID, job.documentID)
}

func processKnowledgeDocumentSyncDelete(job knowledgeSyncJob) error {
	if job.db == nil {
		return fmt.Errorf("数据库连接为空")
	}
	if job.knowledgeSnapshot == nil || job.documentSnapshot == nil {
		return fmt.Errorf("文档删除同步缺少快照")
	}
	return syncKnowledgeDocumentDeleteBestEffort(job.db, *job.knowledgeSnapshot, *job.documentSnapshot)
}
