// tcp_uploader.js
import net from 'net';
import fs from 'fs';
import path from 'path';

if (process.argv.length < 7) {
  console.error('Usage: node tcp_uploader.js <host> <port> <uploadID> <file> <chunkSize>');
  process.exit(1);
}
const [,, HOST, PORT, UPLOAD_ID, FILE_PATH, CHUNK_SIZE_S] = process.argv;
const CHUNK_SIZE = parseInt(CHUNK_SIZE_S, 10);

// helper: send one framed message (len + JSON header [+ bodyBuffer])
function sendFrame(socket, headerObj, bodyBuffer = null) {
  const headerBuf = Buffer.from(JSON.stringify(headerObj), 'utf8');
  const lenBuf = Buffer.alloc(4);
  lenBuf.writeUInt32BE(headerBuf.length, 0);
  socket.write(lenBuf);
  socket.write(headerBuf);
  if (bodyBuffer) socket.write(bodyBuffer);
}

// Main
(async () => {
  // open a connection to the file 
  const stats = fs.statSync(FILE_PATH);
  // get the total size in bytes
  const totalSize = stats.size;
  // get the no of chunks
  const totalChunks = Math.ceil(totalSize / CHUNK_SIZE);

  // create a tcp connection 
  const socket = net.createConnection({ host: HOST, port: parseInt(PORT, 10) }, () => {
    console.log(`Connected to ${HOST}:${PORT}`);
    // 1) INIT frame
    sendFrame(socket, {
      upload_id: UPLOAD_ID,
      operation_code: 0,
      chunk_no:     0,
      chunk_size:   CHUNK_SIZE
    });
    console.log('→ INIT');

    // 2) Send all chunks
    const stream = fs.createReadStream(FILE_PATH, { highWaterMark: CHUNK_SIZE });
    let chunkNo = 0;
    stream.on('data', chunkBuffer => {
      chunkNo++;
      sendFrame(socket, {
        upload_id:      UPLOAD_ID,
        operation_code: 1,
        chunk_no:       chunkNo,
        chunk_size:     chunkBuffer.length
      }, chunkBuffer);
      console.log(`→ CHUNK ${chunkNo}/${totalChunks} (${chunkBuffer.length} bytes)`);
    });

    stream.on('end', () => {
      console.log('All chunks sent');
      // 3) FINISH frame
      sendFrame(socket, {
        upload_id:      UPLOAD_ID,
        operation_code: 2,
        chunk_no:       0,
        chunk_size:     0
      });
      console.log('→ FINISH');
    });
  });

  // log any data the server pushes back (ACKs / errors)
  socket.on('data', buf => {
    // assume server always sends JSON per frame
    try {
      const str = buf.toString('utf8').trim();
      console.log('←', JSON.parse(str));
    } catch {
      console.log('← [non-JSON]', buf.toString('hex'));
    }
  });

  socket.on('end', () => {
    console.log('Connection closed by server');
  });
  socket.on('error', err => {
    console.error('Socket error:', err);
  });
})();
