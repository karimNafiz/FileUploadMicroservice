// Package chunk_job provides a singleton buffered channel and separate pools of
// workers (goroutines) to process ChunkJob tasks concurrently and handle errors.
// It decouples the fast write pipeline from error handling logic.
package chunk_job

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	//p_safemap "github.com/file_upload_microservice/safemap"
	//p_upload_session "github.com/file_upload_microservice/upload_session"
	// logger "docTrack/logger"
)

// -----------------------------------------------------------------------------
// ChunkJob
// -----------------------------------------------------------------------------

// ChunkJob represents a single unit of work: a chunk of bytes that needs
// to be written to disk (or elsewhere). All the information needed to
// write the chunk is carried in this struct.
type ChunkJob struct {
	uploadID   string // Unique ID for the overall upload session
	parentPath string // Base directory path where chunk files should be stored
	chunkNO    uint   // Sequence number of this chunk within the upload
	data       []byte // Raw payload bytes for this chunk

	ack_channel chan<- *ChunkJobAck
	err_channel chan<- *ChunkJobError
}

// String returns a concise description of the ChunkJob for logging.
func (job *ChunkJob) String() string {
	return fmt.Sprintf("ChunkJob(upload=%s, chunk=%d)", job.uploadID, job.chunkNO)
}

// need to get the chunk file name
// so there would be a base directory like tempUploadDir
// then in there there would be folders named according to the uploadIDS
// inside that folder we will have chunk files name chunk_chunkNO
// I might need this now im jus gonna use the String method
// func (job *ChunkJob) getChunkFileName() string {
// 	return fmt.Sprintf("chunkNO")
// }

func CreateChunkJob(uploadID string, chunkNO uint, baseDirectoryPath string, data []byte, ack_channel chan<- *ChunkJobAck, err_channel chan<- *ChunkJobError) *ChunkJob {
	// create the parentPath
	parentPath := filepath.Join(baseDirectoryPath, uploadID)
	return &ChunkJob{
		uploadID:    uploadID,
		chunkNO:     chunkNO,
		parentPath:  parentPath,
		data:        data,
		ack_channel: ack_channel,
		err_channel: err_channel,
	}

}

// in the error struct I have the uploadID public
// in the original one I have it private
// most probably change this in the near future
type ChunkJobError struct {
	UploadID string
	ChunkNo  uint
	Error    error
}

func (c *ChunkJobError) MarshalJSON() ([]byte, error) {
	body := struct {
		UploadID string `json:"uploadID"`
		ChunkNo  uint   `json:"chunk_no"`
		Error    string `json:"error"`
	}{
		UploadID: c.UploadID,
		ChunkNo:  c.ChunkNo,
		Error:    c.Error.Error(),
	}
	return json.Marshal(body)

}

// have a struct for chunk Job Acks
// might change in the future for something better

type ChunkJobAck struct {
	UploadID string
	ChunkNo  uint
}

// MarshalJSON adds a custom message field to the output
func (c ChunkJobAck) MarshalJSON() ([]byte, error) {
	type alias struct {
		UploadID string `json:"uploadID"`
		ChunkNo  uint   `json:"chunk_no"`
		Status   string `json:"status"`
	}

	return json.Marshal(alias{
		UploadID: c.UploadID,
		ChunkNo:  c.ChunkNo,
		Status:   "ok",
	})
}

// func main() {
// 	ack := ChunkJobAck{
// 		UploadID: "xyz789",
// 		ChunkNo:  5,
// 	}
// 	data, _ := json.Marshal(ack)
// 	fmt.Println(string(data))
// }

// need a struct to represent error messages for chunk jobs
// it should contain the uploadID, chunkNO, error_message

// -----------------------------------------------------------------------------
// Singleton buffered channel container
// -----------------------------------------------------------------------------

// bufferedChunkJobChannelStruct wraps two channels:
//   - jobs: for ChunkJobs to write
//   - jobErrors: for jobs that failed writing
//
// This allows separate pipelines for writes and error handling.
type bufferedChunkJobChannelStruct struct {
	jobs            chan *ChunkJob // buffered channel carrying ChunkJob instances
	jobErrors       chan *ChunkJob // buffered channel carrying failed ChunkJob instances
	jobConfirmation chan *ChunkJob
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
			jobs:            make(chan *ChunkJob, bufferSize),
			jobConfirmation: make(chan *ChunkJob, bufferSize),
			jobErrors:       make(chan *ChunkJob, bufferSize),
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
					fmt.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!! signalling the worker thread to stop ")
					return
				case job := <-bufferedChunkJobChannelInstance.jobs:
					// Received a chunk job → attempt to write
					writeChunkAt(job, bufferedChunkJobChannelInstance.jobErrors, bufferedChunkJobChannelInstance.jobConfirmation)
				}
			}
		}(i)
	}

	return nil
}

func StartJobConfirmationHandlerPool(ctx context.Context, handlerCount uint) error {
	if bufferedChunkJobChannelInstance == nil {
		return fmt.Errorf("chunk job confirmation channel not initialized;  call InstantiateBufferedChunkJobChannel first ")
	}

	for i := uint(0); i < handlerCount; i++ {
		go func(handlerID uint) {
			for {
				select {
				case <-ctx.Done():
					// if the context is closed we just return and the go routine is stopped
					return
				case confirmedJob := <-bufferedChunkJobChannelInstance.jobConfirmation:
					handle_job_confirmation(confirmedJob)
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
					handle_job_failed(failedJob)
				}
			}
		}(i)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Enqueue API
// -----------------------------------------------------------------------------

// AddChunkJob enqueues one ChunkJob into the jobs buffer. If the buffer is full,
// this call blocks until a worker frees up space. Returns an error if channels
// aren't initialized.
func AddChunkJob(job *ChunkJob) error {
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
func writeChunkAt(job *ChunkJob, errChannel chan<- *ChunkJob, confirmedChannel chan<- *ChunkJob) {
	// so the filePath would be tempUploadDir/uploadID(ChunkJob)/ChunkJob.String() whatever string returns
	filePath := filepath.Join(job.parentPath, job.String())

	// Ensure parent directory exists, creating any missing folders.
	// TODO: maybe optimize this step,
	// if all the folders exist, doesnt return any error
	if err := os.MkdirAll(job.parentPath, 0755); err != nil {
		// Log and push the failed job onto jobErrors
		//logger.ErrorLogger.Printf("[%s] mkdir error: %v", job.String(), err)
		fmt.Println("error writing chunk job ")
		fmt.Println(err.Error())
		errChannel <- job
		return
	}

	// Open or truncate the file for writing.
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		//logger.ErrorLogger.Printf("[%s] open file error: %v", job.String(), err)
		fmt.Println("error writing chunk job ")
		fmt.Println(err.Error())
		errChannel <- job
		return
	}
	defer f.Close()

	// Write the chunk data to disk.
	if _, err := f.Write(job.data); err != nil {
		//logger.ErrorLogger.Printf("[%s] write error: %v", job.String(), err)
		fmt.Println("error writing chunk job ")
		fmt.Println(err.Error())
		errChannel <- job
		return
	}
	// Success: log the successful write.
	// if the write is successful
	// then we will push this to the jobConfirmedPool
	fmt.Println("chunk job successfully written onto disk")
	fmt.Println(job.String())
	confirmedChannel <- job

}

func handle_job_confirmation(chunk_job *ChunkJob) {
	chunk_job.ack_channel <- &ChunkJobAck{UploadID: chunk_job.uploadID, ChunkNo: chunk_job.chunkNO}

}

func handle_job_failed(chunk_job *ChunkJob) {
	chunk_job.err_channel <- &ChunkJobError{UploadID: chunk_job.uploadID, ChunkNo: chunk_job.chunkNO}
}

// -----------------------------------------------------------------------------
// Consumer logic (errors)
// -----------------------------------------------------------------------------

// !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!! depricated code !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

// handleFailedJob processes a job that failed writing. You can implement:
// - retry logic (e.g., re-enqueue after backoff)
// - updating database status or metrics
// - alerting or logging details
// func handleFailedJob(job *ChunkJob) {
// 	// TODO: implement retry policies, metrics, or DB updates
// 	//logger.ErrorLogger.Printf("handling failed job: %s", job.String())

// }

// might have to review this architecture
// because currently handleConfirmedJob doesnt seem like it deserves it own channel
// func handleConfirmedJob(job *ChunkJob, safemap *p_safemap.SafeMap[*p_upload_session.UploadSession]) {
// 	// notify the safemap chunk job complete
// 	val, exists := safemap.Get(job.uploadID)
// 	if !exists {
// 		// dont really know what to do
// 		// maybe we jus log
// 		return
// 	}
// 	val.NotifyConfirmation()

// }

// need the function create chunk job

// func GetFuncOnAck(ack_channel chan<- *ChunkJobAck) func(*ChunkJob) {

// 	return func(chunk_job *ChunkJob) {

// 		chunk_job_ack := &ChunkJobAck{
// 			UploadID: chunk_job.uploadID,
// 			ChunkNo:  chunk_job.chunkNO,
// 		}
// 		ack_channel <- chunk_job_ack
// 	}

// }
