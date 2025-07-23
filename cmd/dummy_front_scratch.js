import net from 'net';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const file_name = 'test.pdf';
const file_path = path.join(__dirname, 'test_uploads', file_name);
// TODO: right now there is not timeout checks in the code at all, must implement those in version 2

//
// const file_path = '/test_uploads/test.pdf';
// const file_name = 'test.pdf';

// Create a promisified TCP connection with timeout
function Connect(options, timeoutMs) {
    return new Promise((resolve, reject) => {
        const socket = net.createConnection(options);

        const timeoutId = setTimeout(() => {
            socket.destroy();
            reject(new Error(`Connection timed out`));
        }, timeoutMs);

        socket.once('connect', () => {
            clearTimeout(timeoutId);
            resolve(socket);
        });

        socket.once('error', (err) => {
            clearTimeout(timeoutId);
            reject(new Error(`Could not connect to endpoint ${JSON.stringify(options)}: ${err.message}`));
        });
    });
}

// Initialize the upload session by contacting the HTTP service
async function InitUploadSession(file_name, file_path) {
    let chunk_size, total_chunks, uploadID, file_upload_domain, file_upload_port;

    try {
        const stat = fs.statSync(file_path);

        const response = await fetch('http://localhost:8080/upload', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                filename: file_name,
                file_size: stat.size
            })
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status ${response.status}`);
        }

        const respBody = await response.json();
        console.log(`Raw response body:`, respBody);

        ({ uploadID, chunk_size, total_chunks, file_upload_domain, file_upload_port } = respBody);

        console.log(`uploadID: ${uploadID}, chunkSize: ${chunk_size}, totalChunks: ${total_chunks}`);
        console.log(`file_upload_domain: ${file_upload_domain}, file_upload_port: ${file_upload_port}`);
    } catch (err) {
        console.error(`Error connecting to main service: ${err.message}`);
        throw err;
    }

    return {
        uploadID,
        chunk_size,
        total_chunks,
        file_upload_domain,
        file_upload_port,
        file_name,
        file_path
    };
}

// Start the chunked upload process by opening a TCP connection
async function StartChunkedUpload(f_upload_data) {
    const sendFrame = GetSendFrame(4);

    try {
        const socket = await Connect(
            {
                host: f_upload_data.file_upload_domain,
                port: f_upload_data.file_upload_port
            },
            5000
        );

        // Wait for server confirmation after sending the initial message
        const confirmationPromise = new Promise((resolve, reject) => {
            socket.once('data', (buf) => {
                try {
                    const msg = JSON.parse(buf.toString('utf-8').trim());
                    if (msg.status !== 'ok') {
                        return reject(new Error(msg.message || 'Unknown error from server'));
                    }
                    resolve();
                } catch (err) {
                    reject(new Error(`Invalid JSON response: ${err.message}`));
                }
            });
        });

        sendFrame(socket, {
            upload_id: f_upload_data.uploadID,
            operation_code: 0,
            chunk_no: 0,
            chunk_size: f_upload_data.chunk_size
        });

        await confirmationPromise;
        console.log('Initial chunk upload acknowledged by server.');
        // after confirmation we again listen to the sockets on data event
        const acks = new Set();
        // socket.on('data', (buffer)=>{
        //     const msg = parseBufferFrmFServer(buffer)
        //     // TODO: check if msg has the status field or not
        //     // TODO: check if msg has the chunk_no field or not
        //     switch (msg.status){
        //         case "ok":
        //             console.log(`the server has acked back a chunk; chunk ack ${msg}`)
        //             acks.add(msg.chunk_no)
        //
        //             // TODO: implement proper checking if all the chunks are present or not
        //             // TODO: remove this line of code, I'm only using this to test the code
        //             if(acks.size >= f_upload_data.total_chunks){
        //                 sendFrame(socket , {
        //                     upload_id: f_upload_data.uploadID,
        //                     operation_code:2, // hard coding this here
        //                     chunk_no: -1,
        //                     chunk_size: f_upload_data.chunk_size
        //                 })
        //             }
        //
        //             break;
        //         case "error":
        //             console.log(`the server has notified us of an error; chunk error ${msg}`)
        //             // TODO: implement re-transmission
        //             break;
        //     }
        // })// dk if I should put in a lambda function (for closure) or smth else

        // after the initial chunk is acked by the server
        // create a stream to the file
        const fileStream = fs.createReadStream(f_upload_data.file_path , {highWaterMark:f_upload_data.chunk_size})
        let chunk_no = 0
        let operation_code = 1 // check if 1 is the correct operation code
        const baseHeader = {
            upload_id: f_upload_data.uploadID,
            operation_code: operation_code,
            chunk_no:chunk_no,
            chunk_size: f_upload_data.chunk_size
        }
        fileStream.on('data', (buff)=>{
            // create a header
            // massive waste in memory
            // TODO: read about the memory layout of node js
            // TODO: check if the buffer size matches chunk size
            chunk_no++;
            baseHeader.chunk_no = chunk_no
            console.log(`sending chunk no -> ${chunk_no}`)
            sendFrame(socket , baseHeader , buff)
            // and send the frame
        })
        fileStream.on('end' , ()=>{
            console.log("fileStream has been closed ")
        }) // right now I don't have the energy to fix this at all
        fileStream.on('error', ()=>{}) // just throw an error and get the fuck out of it


        // TODO: Implement loop to send actual file chunks...
        // You would slice the file using fs.createReadStream or Buffer and send them similarly.

    } catch (err) {
        console.error(`Could not start chunked upload: ${err.message}`);
    }
}


function parseBufferFrmFServer(buffer){
    try{
        return JSON.parse(buffer.toString('utf-8').trim());
    }catch(err){
        console.log(`func:parseBufferFrmFServer, couldn't parse the buffer from the server ${err}`)
        throw new Error(err)
    }
}












// Entrypoint
async function UploadFiles(file_name, file_path) {
    try {
        const f_upload_data = await InitUploadSession(file_name, file_path);
        await StartChunkedUpload(f_upload_data);
    } catch (err) {
        console.error(`Upload failed: ${err.message}`);
    }
}

// Frame builder function for sending headers + body
function GetSendFrame(header_len_fixed) {
    return function sendFrame(socket, headerObj, bodyBuffer = null) {
        const headerBuf = Buffer.from(JSON.stringify(headerObj), 'utf8');
        const lenBuf = Buffer.alloc(header_len_fixed);
        lenBuf.writeUInt32BE(headerBuf.length, 0);
        socket.write(lenBuf);
        socket.write(headerBuf);
        if (bodyBuffer) socket.write(bodyBuffer);
    };
}

UploadFiles(file_name , file_path)

