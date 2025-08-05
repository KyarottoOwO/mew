package main

import (
	_ "Cura/modules"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	webview "github.com/webview/webview_go"
)

var Main string

func main() {
	Main = `
	<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <script src="https://cdn.tailwindcss.com"></script>
    <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css" rel="stylesheet">
    <script src="https://cdn.jsdelivr.net/npm/sweetalert2@11"></script>

</head>
<style>
    :root {
        --main-color: #57b4c7;
    }

    body {
        background-color: black;
        color: white;
        overflow: hidden;
    }

    .text-main {
        color: var(--main-color);
    }

    .border-main {
        border-color: var(--main-color);
    }

    .bg-main {
        background-color: var(--main-color);
    }

    .hover\:bg-main:hover {
        background-color: var(--main-color);
    }

    .drag-over {
        border-color: var(--main-color) !important;
        background-color: rgba(87, 180, 199, 0.1);
    }

    .swal-custom-popupmenu {
        width: 30vw;
        height: 35vw;
        background-color: #050505 !important;
        border-radius: 0;
        border: 1px solid #171717 !important;
        color: white !important;

    }

    .swal-custom-popupmenu a {
        color: #4ca2ca;
        text-decoration: underline;
    }


    .swal-custom-popupmenu .swal2-title {
        color: #4ca2ca;
    }

    .swal-custom-popup {
        width: 30vw;
        height: 35vw;
        background-color: #050505 !important;
        border-radius: 0;
        border: 1px solid #171717 !important;
        color: white !important;
    }

    .custom-confirm-btn {
        background-color: #050505;
        color: white;
        border: 1px solid #171717;
        padding: 5px 20px;
        border-radius: 0;
        font-size: 16px;
    }

    .custom-confirm-btn:hover {
        background-color: white;
        color: black;
    }
</style>
</head>

<body>
    <div class="bg-black text-white flex justify-center items-center h-screen">
        <div
            class="fixed left-[50%] top-[50%] z-50 grid max-h-screen w-[512px] h-[488px] translate-x-[-50%] translate-y-[-50%] gap-3 border border-neutral-900 bg-[hsl(0_0%_2%)] p-6 pb-4 shadow-lg">
            <div class="absolute top-4 right-8">
                <img id="logo" src="https://files.catbox.moe/oxcos9.png" alt="Top Right Image" class="w-12 h-12">
            </div>
            <div class="flex justify-between items-center mt-1">
                <h2 class="text-lg leading-none tracking-tight flex items-center gap-4 font-semibold">Upload Pack</h2>
            </div>
            <p class="text-sm text-neutral-400 mb-4">Drag and drop your file here or click to upload.</p>
            <label for="fileUpload" id="dropZone"
                class="relative w-[455px] h-[194px] border border-main flex items-center justify-center p-4 pb-6 cursor-pointer group"
                style="border: 1px dashed hsla(220, 50%, 56%, 0.2); background-color: hsla(220, 50%, 56%, 0.025); padding: 8px;">
                <input id="fileUpload" type="file" class="hidden" />
                <div id="initialBox"
                    class="relative z-40 mx-auto flex h-32 w-full max-w-[8rem] items-center justify-center border border-neutral-900 bg-neutral-950 group-hover:shadow-2xl shadow-[0px_10px_50px_rgba(0,0,0,0.1)] transition-all duration-300 group-hover:opacity-[0.90] group-hover:translate-x-[20px] group-hover:translate-y-[-20px] group-hover:scale-105">
                    <svg stroke="currentColor" fill="none" stroke-width="2" viewBox="0 0 24 24" stroke-linecap="round"
                        stroke-linejoin="round" class="h-4 w-4 text-neutral-300" height="1em" width="1em"
                        xmlns="http://www.w3.org/2000/svg">
                        <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                        <polyline points="17 8 12 3 7 8"></polyline>
                        <line x1="12" x2="12" y1="3" y2="15"></line>
                    </svg>
                </div>

                <div id="fileInfoBox" class="hidden w-full">
                    <div
                        class="relative z-40 mx-auto mt-4 flex w-fit max-w-[220px] flex-col items-start justify-start overflow-hidden bg-neutral-900 p-4 sm:max-w-xs shadow-sm">
                        <div class="flex w-full items-center justify-between gap-4">
                            <p id="fileName" class="max-w-xs truncate text-base text-neutral-300"></p>
                            <p id="fileSize"
                                class="w-fit flex-shrink-0 bg-neutral-800 px-2 py-1 text-sm text-white shadow-input">
                            </p>
                        </div>
                        <div
                            class="mt-2 flex w-full flex-col items-start justify-between text-sm text-neutral-400 md:flex-row md:items-center">
                            <p id="fileType" class="bg-neutral-800 px-1 py-0.5"></p>
                            <p id="fileModified"></p>
                        </div>
                    </div>
                </div>
                <div id="hoverBox"
                    class="absolute inset-0 z-30 mx-auto mt-8 flex h-32 w-full max-w-[8rem] items-center justify-center border bg-[hsl(0_0%_2%)] opacity-0 transition-opacity duration-300 group-hover:opacity-100"
                    style="border: 1px dashed hsla(220, 50%, 56%, 0.2);">
                </div>

            </label>
            <p class="text-sm text-neutral-400 mb-7">After uploading click confirm to start porting your pack.</p>
            <div class="flex items-center justify-between w-full">
                <div class="flex flex-col items-start">
                </div>

                <div class="flex items-center gap-2 mt-7">
                    <button id="cancel"
                        class="flex font-medium justify-center items-center text-sm gap-2 px-4 py-2 border border-neutral-700 text-white hover:bg-white hover:text-black h-[34px]">Cancel</button>
                    <button id="confirm"
                        class="flex font-medium justify-center items-center text-sm gap-2 px-4 py-2 border border-main text-main hover:bg-main hover:text-black h-[34px]">Confirm</button>
                </div>
            </div>
        </div>
    </div>
    <script>
        FinishPort();

        const logo = document.getElementById("logo");
        const cancel = document.getElementById("cancel");
        const confirm = document.getElementById("confirm");
        const fileUpload = document.getElementById("fileUpload");
        const initialBox = document.getElementById("initialBox");
        const fileInfoBox = document.getElementById("fileInfoBox");
        const hoverBox = document.getElementById("hoverBox");
        const fileName = document.getElementById("fileName");
        const fileSize = document.getElementById("fileSize");
        const fileType = document.getElementById("fileType");
        const fileModified = document.getElementById("fileModified");
        const dropZone = document.getElementById("dropZone");
        let file;


        dropZone.addEventListener("drop", (event) => {
            event.preventDefault();
            dropZone.classList.remove("drag-over");
            const file = event.dataTransfer.files[0];
            fileUpload.files = event.dataTransfer.files;
            HandleFile(file);
        });

        dropZone.addEventListener("dragover", (event) => {
            event.preventDefault();

            dropZone.classList.add("drag-over");
        });

        dropZone.addEventListener("dragleave", () => {
            dropZone.classList.remove("drag-over");
        });

        function HandleFile(File) {
            file = File;
            if (file) {
                initialBox.classList.add("hidden");
                hoverBox.classList.add("hidden");
                fileInfoBox.classList.remove("hidden");
                fileName.textContent = file.name;
                fileSize.textContent = (file.size / 1024 / 1024).toFixed(2)} + " MB";
                fileType.textContent = file.type;
                fileModified.textContent = "modified" + new Date(file.lastModified).toLocaleDateString();
        }

        fileUpload.addEventListener("change", (eva) => {
            HandleFile(eva.target.files[0]);
        });

        function PopupMenu() {
            const ws = new WebSocket("/ws/PopupMenu");
            ws.onmessage = (event) => {
                const message = JSON.parse(event.data);
                Swal.mixin({
                    customClass: {
                        popup: 'swal-custom-popupmenu',
                        confirmButton: 'custom-confirm-btn'
                    },
                }).fire({
                    title: "Cura",
                    html: 'Cura is a tool designed to help people port or create texture packs for <strong>Minecraft: Bedrock Edition</strong>. It is developed by <strong>Kyarotto</strong>, and the UI is designed by <strong>zkyrosz</strong>. Cura is still in development and may contain bugs or issues. Please report any problems to the <a href="https://discord.gg/XMQXn3eW2D" target="_blank" rel="noopener noreferrer">Discord server</a>.',
                    confirmButtonText: "Got it!",
                });
            };
        }


        logo.addEventListener("click", () => {
            PopupMenu()
        });

        cancel.addEventListener("click", () => {
            console.log("Cancel");
            fileUpload.value = "";
            initialBox.classList.remove("hidden");
            hoverBox.classList.remove("hidden");
            fileInfoBox.classList.add("hidden");
            fileName.textContent = "";
            fileSize.textContent = "";
            fileType.textContent = "";
            fileModified.textContent = "";
            file = null;
        });

        function arrayBufferToBase64(buffer) {
            let binary = '';
            const bytes = new Uint8Array(buffer);
            const len = bytes.byteLength;
            for (let i = 0; i < len; i++) {
                binary += String.fromCharCode(bytes[i]);
            }
            return window.btoa(binary);
        }

        function portPack(PackBytes) {
            const ws = new WebSocket("/ws/PortPack");
            ws.onopen = function () {
                const base64Bytes = arrayBufferToBase64(PackBytes);
                const message = {
                    bytes: base64Bytes,
                    text: file.name
                };
                ws.send(JSON.stringify(message));
            };
        }

function FinishedPack() {
    const ws = new WebSocket("/ws/FinishedPack");

    ws.onopen = () => {
        console.log("FinishedPack WebSocket connection established");
    };

    ws.onmessage = (event) => {
        try {
            const message = JSON.parse(event.data);
            console.log("Message received:", message);
            Popup("Porting Finished", message.message + " Has been ported ! ", "success", "OK");
        } catch (error) {
            console.error("Error parsing message:", error);
        }
    };

    ws.onerror = (error) => {
        console.error("WebSocket error:", error);
    };

    ws.onclose = () => {
        console.log("FinishedPack WebSocket closed");
    };
}

confirm.addEventListener("click", () => {
    if (fileType.innerHTML !== "application/x-zip-compressed") {
        console.log(fileType.innerHTML);
        Popup("Error", "Please upload a zip file", "error", "OK");
        return;
    }

    Popup("Porting", file.name + " is being ported, please wait...", "info", "OK");

    const reader = new FileReader();
    reader.readAsArrayBuffer(file);
    reader.onload = function () {
        portPack(reader.result);
    };

    FinishedPack();
});



        function FinishPort() {
            const ws = new WebSocket("/ws/FinishedPack")
            ws.onmessage = (event) => {
                const message = JSON.parse(event.data);

                console.log(message);
            }
        }


        function Popup(title, text, icon, confirmButtonText) {
            Swal.mixin({
                customClass: {
                    popup: 'swal-custom-popup',
                    confirmButton: 'custom-confirm-btn'
                },
            }).fire({
                title: title,
                text: text,
                icon: icon,
                confirmButtonText: confirmButtonText,
            });
        }
    </script>

</body>

</html>
	`

	OpenPort := findOpenPort()
	go func() {
		fmt.Println("Starting server on port " + strconv.Itoa(OpenPort))
		http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			http.ServeContent(writer, request, "index.html", time.Now(), strings.NewReader(Main))
		})
		if err := http.ListenAndServe("127.0.0.1:"+strconv.Itoa(OpenPort), nil); err != nil {
			fmt.Printf("Error starting server: %s", err)
			os.Exit(1)
		}
	}()

	w := webview.New(true)

	w.SetTitle("Cura ᨀ github.com/kyarottoOwO")
	w.SetSize(1200, 800, webview.HintFixed)

	defer w.Destroy()

	w.Dispatch(func() {
		w.Navigate("http://127.0.0.1:" + strconv.Itoa(OpenPort) + "/Cura.HTML")
	})

	w.Run()
}

func findOpenPort() int {
	for i := 1932; i < 65535; i++ {
		conn, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(i))
		if err != nil {
			continue
		}
		_ = conn.Close()
		return i
	}
	return -1
}
