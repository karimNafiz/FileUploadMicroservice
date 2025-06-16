// Package chunk_job provides a singleton buffered channel and separate pools of
// workers (goroutines) to process chunkJob tasks concurrently and handle errors.
// It decouples the fast write pipeline from error handling logic.
package chunk_job

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	// logger "docTrack/logger"
)

// -----------------------------------------------------------------------------
// chunkJob
// -----------------------------------------------------------------------------

// chunkJob represents a single unit of work: a chunk of bytes that needs
// to be written to disk (or elsewhere). All the information needed to
// write the chunk is carried in this struct.
type chunkJob struct {
	uploadID   string // Unique ID for the overall upload session
	parentPath string // Base directory path where chunk files should be stored
	chunkNO    uint   // Sequence number of this chunk within the upload
	data       []byte // Raw payload bytes for this chunk
}

// String returns a concise description of the chunkJob for logging.
func (job *chunkJob) String() string {
	return fmt.Sprintf("chunkJob(upload=%s, chunk=%d)", job.uploadID, job.chunkNO)
}

// need to get the chunk file name
// so there would be a base directory like tempUploadDir
// then in there there would be folders named according to the uploadIDS
// inside that folder we will have chunk files name chunk_chunkNO
// I might need this now im jus gonna use the String method
// func (job *chunkJob) getChunkFileName() string {
// 	return fmt.Sprintf("chunkNO")
// }

func CreateChunkJob(uploadID string, chunkNO uint, baseDirectoryPath string, data []byte) *chunkJob {
	// create the parentPath
	parentPath := filepath.Join(baseDirectoryPath, uploadID)
	return &chunkJob{
		uploadID:   uploadID,
		chunkNO:    chunkNO,
		parentPath: parentPath,
		data:       data,
	}

}

// -----------------------------------------------------------------------------
// Singleton buffered channel container
// -----------------------------------------------------------------------------

// bufferedChunkJobChannelStruct wraps two channels:
//   - jobs: for chunkJobs to write
//   - jobErrors: for jobs that failed writing
//
// This allows separate pipelines for writes and error handling.
type bufferedChunkJobChannelStruct struct {
	jobs      chan *chunkJob // buffered channel carrying chunkJob instances
	jobErrors chan *chunkJob // buffered channel carrying failed chunkJob instances
}

var (
	// bufferedChunkJobChannelInstance holds the singleton instance.
	bufferedChunkJobChannelInstance *bufferedChunkJobChannelStruct

	// onceBufferChannel ensures the singleton is only created once.
	onceBufferChannel sync.Once
)

// InstantiateBufferedChunkJobChannel initializes the singleton buffered channel
// with the specified capacity for both jobs and jobErrors. Safe to call multiple
// times—only the first call creates the channels.
func InstantiateBufferedChunkJobChannel(bufferSize uint) {
	onceBufferChannel.Do(func() {
		bufferedChunkJobChannelInstance = &bufferedChunkJobChannelStruct{
			jobs: make(chan *chunkJob, bufferSize),
			// match error channel size to job buffer to avoid blocking writers
			jobErrors: make(chan *chunkJob, bufferSize),
		}
	})
}

// -----------------------------------------------------------------------------
// Worker pool management
// -----------------------------------------------------------------------------

// StartWorkerPool launches 'poolSize' goroutines that continuously read from
// the jobs channel and attempt to write each chunk. Failed jobs are sent to
// the jobErrors channel for separate handling. Workers exit when ctx is canceled.
func StartWorkerPool(ctx context.Context, poolSize uint) error {
	if bufferedChunkJobChannelInstance == nil {
		return fmt.Errorf("chunk job channel not initialized; call InstantiateBufferedChunkJobChannel first")
	}

	// Launch each writer worker.
	for i := uint(0); i < poolSize; i++ {
		go func(workerID uint) {
			for {
				select {
				case <-ctx.Done():
					// Graceful shutdown: stop processing when context is canceled
					return
				case job := <-bufferedChunkJobChannelInstance.jobs:
					// Received a chunk job → attempt to write
					writeChunkAt(job, bufferedChunkJobChannelInstance.jobErrors)
				}
			}
		}(i)
	}

	return nil
}

// StartErrorHandlerPool launches 'handlerCount' goroutines that read from the
// jobErrors channel and perform error-specific logic (e.g., retries, DB updates,
// alerts). They exit when ctx is canceled.
func StartErrorHandlerPool(ctx context.Context, handlerCount uint) error {
	if bufferedChunkJobChannelInstance == nil {
		return fmt.Errorf("chunk job channel not initialized; call InstantiateBufferedChunkJobChannel first")
	}

	for i := uint(0); i < handlerCount; i++ {
		go func(handlerID uint) {
			for {
				select {
				case <-ctx.Done():
					// Shutdown signal received
					return
				case failedJob := <-bufferedChunkJobChannelInstance.jobErrors:
					// Process the failed job (e.g., retry or mark failed in DB)
					handleFailedJob(failedJob)
				}
			}
		}(i)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Enqueue API
// -----------------------------------------------------------------------------

// AddChunkJob enqueues one chunkJob into the jobs buffer. If the buffer is full,
// this call blocks until a worker frees up space. Returns an error if channels
// aren't initialized.
func AddChunkJob(job *chunkJob) error {
	if bufferedChunkJobChannelInstance == nil {
		return fmt.Errorf("chunk job channel not initialized; call InstantiateBufferedChunkJobChannel first")
	}

	//logger.InfoLogger.Println("Added Chunk Job " + job.String())

	bufferedChunkJobChannelInstance.jobs <- job
	return nil
}

// -----------------------------------------------------------------------------
// Consumer logic (write)
// -----------------------------------------------------------------------------

// writeChunkAt attempts to write the chunk to disk. On error, it sends the job
// into the provided errChannel for separate handling.
func writeChunkAt(job *chunkJob, errChannel chan<- *chunkJob) {
	// so the filePath would be tempUploadDir/uploadID(chunkJob)/chunkJob.String() whatever string returns
	filePath := filepath.Join(job.parentPath, job.String())

	// Ensure parent directory exists, creating any missing folders.
	// TODO: maybe optimize this step,
	// if all the folders exist, doesnt return any error
	if err := os.MkdirAll(job.parentPath, 0755); err != nil {
		// Log and push the failed job onto jobErrors
		//logger.ErrorLogger.Printf("[%s] mkdir error: %v", job.String(), err)
		errChannel <- job
		return
	}

	// Open or truncate the file for writing.
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		//logger.ErrorLogger.Printf("[%s] open file error: %v", job.String(), err)
		errChannel <- job
		return
	}
	defer f.Close()

	// Write the chunk data to disk.
	if _, err := f.Write(job.data); err != nil {
		//logger.ErrorLogger.Printf("[%s] write error: %v", job.String(), err)
		errChannel <- job
		return
	}

	// Success: log the successful write.
	//logger.InfoLogger.Printf("[%s] successfully written", job.String())
}

// -----------------------------------------------------------------------------
// Consumer logic (errors)
// -----------------------------------------------------------------------------

// handleFailedJob processes a job that failed writing. You can implement:
// - retry logic (e.g., re-enqueue after backoff)
// - updating database status or metrics
// - alerting or logging details
func handleFailedJob(job *chunkJob) {
	// TODO: implement retry policies, metrics, or DB updates
	//logger.ErrorLogger.Printf("handling failed job: %s", job.String())
}

// need the function create chunk job
