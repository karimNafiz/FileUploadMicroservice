// import { readFile as readFilePromise } from 'fs/promises';
// import { readFile as readFileCallback } from 'fs';
// import { promisify } from 'util';
//
// // Convert callback-style readFile into a Promise-based version
// const pReadFile = promisify(readFileCallback);
//
// // Version using async/await with fs/promises
// async function readData() {
//     try {
//         const data = await readFilePromise("test.txt", "utf8");
//         console.log(data);
//         console.log("hi this is the second line");
//     } catch (err) {
//         console.error(err);
//     }
// }
//
// // Version using manually promisified fs.readFile
// function pReadData() {
//     pReadFile("test.txt", "utf8")
//         .then(data => {
//             console.log(data);
//             console.log("hi this is the second line test");
//         })
//         .catch(err => console.error(err));
//     console.log("hi this is the second line ")
// }
//
// // Call whichever version you want
// pReadData();
// // or
// // pReadData();
// console.log("1");
//
// setTimeout(() => console.log("2 (timeout)"), 0);
//
// (async () => {
//     const result = await new Promise(resolve => setTimeout(() => resolve("3 (awaited)"), 0));
//     console.log(result);
// })();
//
// console.log("4");
