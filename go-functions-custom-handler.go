package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
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

func main() {
	customHandlerPort, exists := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if exists {
		fmt.Println("FUNCTIONS_CUSTOMHANDLER_PORT: " + customHandlerPort)
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/api/SimpleHttpTrigger", simpleHttpTriggerHandler)
	mux.HandleFunc("/QueueTrigger", queueTriggerHandler)
	fmt.Println("Go server Listening...on FUNCTIONS_CUSTOMHANDLER_PORT:", customHandlerPort)
	log.Fatal(http.ListenAndServe(":"+customHandlerPort, mux))
}
