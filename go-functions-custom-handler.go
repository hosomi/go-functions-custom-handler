package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"golang.org/x/image/draw"
)

func simpleHttpTriggerHandler(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	fmt.Println(t.Month())
	fmt.Println(t.Day())
	fmt.Println(t.Year())
	ua := r.Header.Get("User-Agent")
	fmt.Printf("user agent is: %s \n", ua)
	invocationid := r.Header.Get("X-Azure-Functions-InvocationId")
	fmt.Printf("invocationid is: %s \n", invocationid)

	queryParams := r.URL.Query()

	for k, v := range queryParams {
		fmt.Println("k:", k, "v:", v)
	}

	w.Write([]byte("SimpleHttpTriggerHandler from Go lang 💩"))
}

type InvokeRequest struct {
	Data map[string]interface{}
}

type InvokeResponse struct {
	Outputs     map[string]interface{} // function.json ファイルの bindings 配列によって定義される応答値。
	Logs        []string               // Functions の呼び出しログとして表示するメッセージ。
	ReturnValue interface{}            // レスポンス本文。(function.json ファイルの $return として出力が構成されている場合)
}

type User struct {
	Id   int
	Name string
}

type ImageHalf struct {
	Container string
	Directory string
}

func queueTriggerHandler(w http.ResponseWriter, r *http.Request) {
	var invokeReq InvokeRequest
	d := json.NewDecoder(r.Body)
	decodeErr := d.Decode(&invokeReq)
	if decodeErr != nil {
		http.Error(w, decodeErr.Error(), http.StatusBadRequest)
		return
	}
	fmt.Println("invokeReq.Data: ", invokeReq.Data)
	fmt.Println("invokeReq.Data[value]: ", invokeReq.Data["value"])

	data := invokeReq.Data["value"].(string)
	s, _ := strconv.Unquote(string(data))

	u := new(User)
	err := json.Unmarshal([]byte(s), u)
	if err != nil {
		http.Error(w, decodeErr.Error(), http.StatusBadRequest)
	}
	fmt.Printf("%+v\n", u)
	fmt.Println("id:", u.Id, "name:", u.Name)

	// direction: "out" を一つ以上定義しないとカスタムハンドラーは成功しても Functions はタイムアウトでエラーになる。
	invokeResponse := InvokeResponse{Logs: []string{"success"}}
	js, err := json.Marshal(invokeResponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func imageHalf(w http.ResponseWriter, r *http.Request) {

	var invokeReq InvokeRequest
	d := json.NewDecoder(r.Body)
	decodeErr := d.Decode(&invokeReq)
	if decodeErr != nil {
		http.Error(w, decodeErr.Error(), http.StatusBadRequest)
		return
	}
	fmt.Println("invokeReq.Data: ", invokeReq.Data)
	fmt.Println("invokeReq.Data[value]: ", invokeReq.Data["value"])

	data := invokeReq.Data["value"].(string)
	s, _ := strconv.Unquote(string(data))

	ih := new(ImageHalf)
	err := json.Unmarshal([]byte(s), ih)
	if err != nil {
		http.Error(w, decodeErr.Error(), http.StatusBadRequest)
	}
	fmt.Printf("%+v\n", ih)
	fmt.Println("Container:", ih.Container, "Directory:", ih.Directory)

	accountName, accountKey := os.Getenv("AZURE_STORAGE_ACCOUNT"), os.Getenv("AZURE_STORAGE_ACCESS_KEY")
	if len(accountName) == 0 || len(accountKey) == 0 {
		http.Error(w, "Either the AZURE_STORAGE_ACCOUNT or AZURE_STORAGE_ACCESS_KEY environment variable is not set", http.StatusInternalServerError)
		return
	}

	// emulator
	URL, _ := url.Parse(
		fmt.Sprintf("http://127.0.0.1:10000/%s/%s", accountName, ih.Container))

	// fmt.Println("URL", URL)

	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Fatal()
		http.Error(w, "Invalid credentials with error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	containerURL := azblob.NewContainerURL(*URL, p)

	// download
	blobURL := containerURL.NewBlockBlobURL(fmt.Sprintf("%s/%s", ih.Directory, "image.jpg"))
	ctx := context.Background() // never-expiring context
	downloadResponse, err := blobURL.Download(ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	imageMemory, err := jpeg.Decode(downloadResponse.Body(azblob.RetryReaderOptions{MaxRetryRequests: 20}))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// fmt.Println("imageMemory", imageMemory)
	rct := imageMemory.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, rct.Dx()/2, rct.Dy()/2))
	draw.CatmullRom.Scale(dst, dst.Bounds(), imageMemory, rct, draw.Over, nil)

	// upload
	blobUploadURL := containerURL.NewBlockBlobURL(fmt.Sprintf("%s/%s", ih.Directory, "image-half.jpg"))
	buffer := new(bytes.Buffer)
	err = jpeg.Encode(buffer, dst, &jpeg.Options{Quality: 90})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	imageBytes := buffer.Bytes()

	_, err = azblob.UploadBufferToBlockBlob(ctx, imageBytes, blobUploadURL,
		azblob.UploadToBlockBlobOptions{
			BlockSize:   4 * 1024 * 1024,
			Parallelism: 16})

	invokeResponse := InvokeResponse{Logs: []string{"success"}}
	js, err := json.Marshal(invokeResponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func main() {
	customHandlerPort, exists := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if exists {
		fmt.Println("FUNCTIONS_CUSTOMHANDLER_PORT: " + customHandlerPort)
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/api/SimpleHttpTrigger", simpleHttpTriggerHandler)
	mux.HandleFunc("/QueueTrigger", queueTriggerHandler)
	mux.HandleFunc("/ImageHalf", imageHalf)
	fmt.Println("Go server Listening...on FUNCTIONS_CUSTOMHANDLER_PORT:", customHandlerPort)
	log.Fatal(http.ListenAndServe(":"+customHandlerPort, mux))
}
