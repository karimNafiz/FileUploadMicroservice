// tcp_uploader.js
import net from 'net';
import fs from 'fs';

if (process.argv.length < 7) {
  console.error('Usage: node tcp_uploader.js <host> <port> <uploadID> <file> <chunkSize>');
  process.exit(1);
}
const [,, HOST, PORT, UPLOAD_ID, FILE_PATH, CHUNK_SIZE_S] = process.argv;
const CHUNK_SIZE = parseInt(CHUNK_SIZE_S, 10);

// helper: send one framed message (len + JSON header [+ bodyBuffer])
function sendFrame(socket, headerObj, bodyBuffer = null) {
  const headerBuf = Buffer.from(JSON.stringify(headerObj), 'utf8');
  const lenBuf    = Buffer.alloc(4);
  lenBuf.writeUInt32BE(headerBuf.length, 0);
  socket.write(lenBuf);
  socket.write(headerBuf);
  if (bodyBuffer) socket.write(bodyBuffer);
}

(async () => {
  const stats       = fs.statSync(FILE_PATH);
  const totalSize   = stats.size;
  const totalChunks = Math.ceil(totalSize / CHUNK_SIZE);
  
  console.log(`total chunks ${totalChunks}`)
  // keep track of which chunk numbers we've ACKed
  const acks = new Set();

  const socket = net.createConnection(
    { host: HOST, port: parseInt(PORT, 10) },
    () => {
      console.log(`Connected to ${HOST}:${PORT}`);
      // 1) INIT
      sendFrame(socket, {
        upload_id:      UPLOAD_ID,
        operation_code: 0,
        chunk_no:       0,
        chunk_size:     CHUNK_SIZE
      });
      console.log('→ INIT');

      // 2) Fire off all chunks
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
        console.log(`→ CHUNK ${chunkNo}/${totalChunks}`);
      });

      stream.on('end', () => {
        console.log('All chunks sent, awaiting ACKs...');
        // no FINISH here any more!
      });
    }
  );

  socket.on('data', buf => {
    // parse any JSON responses (ack or error)
    let msg;
    try {
      msg = JSON.parse(buf.toString('utf8').trim());
      console.log(`message sent from the file upload service  ${msg}`)
    } catch {
      console.error('← [non-JSON]', buf.toString('hex'));
      return;
    }

    console.log(`ack size ${acks.size}`)
    if (msg.chunk_no && msg.status === 'ok') {
      console.log(`← ACK chunk ${msg.chunk_no}`);
      acks.add(msg.chunk_no);

      // once we've seen every single chunk number 1..totalChunks:
      if (acks.size === totalChunks) {
        console.log('← All ACKs received');
        // 3) Now send FINISH
        sendFrame(socket, {
          upload_id:      UPLOAD_ID,
          operation_code: 2,
          chunk_no:       0,
          chunk_size:     0
        });
        console.log('→ FINISH');
      }
    } else if (msg.chunk_no && msg.status === 'error') {
      console.error(`← ERROR chunk ${msg.chunk_no}: ${msg.message || 'unknown'}`);
      // you could retry here if desired...
    } else if (msg.status === 'complete') {
      console.log('← UPLOAD COMPLETE');
      socket.end();
    } else {
      console.log('←', msg);
    }
  });

  socket.on('end', () => console.log('Connection closed by server'));
  socket.on('error', err => console.error('Socket error:', err));
})();
