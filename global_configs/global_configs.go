package global_configs

import "path/filepath"

const HEADERlENGTH = 4

const UPLOADINITOPCODE = 0
const UPLOADCHUNKOPCODE = 1
const UPLOADFINISHOPCODE = 2
const UPLOADCANCELOPCODE = 3
const UPLOADRETRANSMISSIONOPCODE = 4

const CHUNKJOBWORKERPOOL = 4
const CHUNKJOBERRPOOL = CHUNKJOBWORKERPOOL * 2
const CHUNKJOBCONFIRMATIONPOOL = CHUNKJOBERRPOOL * 2

const CHUNKJOBCONFIRMATIONWORKERPOOL = 1
const CHUNKJOBERRWORKERPOOL = 1

const CHUNKJOBCHANNELBUFFERSIZE = 10

// lets start with 10
// consider increasing it after testing the file upload service with the main file upload service
const SERVICESTATUSNOTIFICATIONCHANNELBUFFER = 5

const TLSCERTDST = "tls"
const TLSCERTNAME = "server.crt"
const TLSKEYNAME = "server.key"

var cHUNKUPLOADROOTFOLDER string = "chunk_uploads"
var uPLOADROOT string = "upload_root"

const SCHEME_HTTP = "http"
const SCHEME_HTTPS = "https"

//var CHUNKUPLOADROOTFOLDERPATH string = filepath.Join(uPLOADROOT, cHUNKUPLOADROOTFOLDER)
func CHUNKUPLOADROOTFOLDERPATH() string {
	return filepath.Join(uPLOADROOT, cHUNKUPLOADROOTFOLDER)
}
