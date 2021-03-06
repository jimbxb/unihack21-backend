package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"github.com/rs/cors"
)

var ModelMap = make(map[int32]ModelMetaData, 1)
var HostMap = make(map[int32]*HostMetaData, 1)
var Hosts = make ([]*HostMetaData, 0)

var ModelCounter int32 = 0


type ModelFeatures struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Encoder string `json:"encoder"`
}

type HostNotFound struct {
	Message string `json:"message"`
	Host string `json:"host"`
}

type ModelMetaData struct {
	ID     int32 `json:"id"`
	Name  string `json:"name"`
	Desc     string `json:"description"`
	InputFeatures ModelFeatures `json:"input_features"`
	OutputFeatures ModelFeatures `json:"output_features"`
}

type HostMetaData struct {
	BFS string `json:"serverId"`
	ModelCount int32 `json:"modelCount"`
}

func createModelHandler(w http.ResponseWriter, r *http.Request) {
	reqBody, _ := ioutil.ReadAll(r.Body)
	var model ModelMetaData
	_ = json.Unmarshal(reqBody, &model)

	model.ID = ModelCounter
	ModelCounter+=1
	ModelMap[model.ID] = model
	_ = json.NewEncoder(w).Encode(model.ID)
}

func getRequestID(r *http.Request) int32 {
	vars := mux.Vars(r)
	id, _ :=  strconv.Atoi(vars["id"])
	numId := int32(id)
	return numId
}

func getNextHost() *HostMetaData {
	best := int32(math.MaxInt32)
	var res *HostMetaData
	for _, host := range Hosts {
		if host.ModelCount < best{
			res = host
			best = host.ModelCount
		}
	}
	return res
}

func uploadModelHandler(w http.ResponseWriter, r *http.Request) {
	reqId := getRequestID(r)
	data := ModelMap[reqId]

	var res interface{}


	req, _ := forwardModel(&data, r)
	client := &http.Client{}
	ret, err := client.Do(req)

	json.NewDecoder(ret.Body).Decode(res)

	fmt.Printf("")
	fmt.Printf("response for model upload: %v", res)

	if err == nil {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(res)
	} else {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(HostNotFound{
			Message: fmt.Sprintf("no response received from requested host, status %v", res),
			Host:    req.URL.String(),
		})
	}
}


func assignModelHost(data *ModelMetaData) string {
	host := getNextHost()
	HostMap[data.ID] = host
	host.ModelCount += 1
	fmt.Println(fmt.Sprintf("assigned %d to %v\n", data.ID, host))
	return host.BFS
}

func forwardModel(data *ModelMetaData, r *http.Request) (*http.Request, error) {
	_ = r.ParseMultipartForm(1 << 31 -1)
	_, header, err :=  r.FormFile("model")
	if err != nil {
		return nil, err
	}
	file, _ := header.Open()
	defer file.Close()
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("model", header.Filename)
	if err != nil {
		return nil, err
	}
	_, _ = io.Copy(part, file)

	metadataFile, _ := json.MarshalIndent(*data, "", "")
	part, err = writer.CreateFormFile("metadata", "metadata.json")

	tmpfile, _:= ioutil.TempFile(os.TempDir(), "tmp-")
	tmpfile.Write(metadataFile)
	defer tmpfile.Close()
	_, _  = io.Copy(part,tmpfile)

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	uri := fmt.Sprintf("%s/load/%d", assignModelHost(data), data.ID)
	return http.NewRequest("POST", uri, body)
}

func getModelsHandler(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(ModelMap)
}

func evalModelHandler(w http.ResponseWriter, r *http.Request) {
	reqId := getRequestID(r)
	modelHost := HostMap[reqId]
	uri := fmt.Sprintf("%s/eval/%d", modelHost.BFS, reqId)

	req, _ := http.NewRequest("POST", uri, r.Body)
	client := &http.Client{}

	ret, err := client.Do(req)

	if err == nil {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(ret.Body)
	} else {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(HostNotFound{
			Message: fmt.Sprintf("no response received from requested host, status %v", err.Error()),
			Host:    req.URL.String(),
		})
	}
}
func trainModelHandler(w http.ResponseWriter, r *http.Request) {

}

func handleRequests() {

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/model", getModelsHandler).Methods("GET")
	router.HandleFunc("/model", createModelHandler).Methods("POST")
	router.HandleFunc("/model/{id}", uploadModelHandler).Methods("POST")
	router.HandleFunc("/eval/{id}", evalModelHandler).Methods("POST")
	router.HandleFunc("/train/{id}", trainModelHandler).Methods("POST")
	handler := cors.Default().Handler(router)
	log.Fatal(http.ListenAndServe(":5000", handler))
}

func makeInit(){
	hostString := os.Getenv("HOSTS")
	hosts := strings.Split(hostString, ",")
	for _, v := range hosts {
		Hosts = append(Hosts, &HostMetaData{
			BFS:        v,
			ModelCount: 0,
		})
	}
	for _, v := range Hosts {
		fmt.Println(v)

	}
}

func main() {
	fmt.Println("listening on port 5000")
	makeInit()
	handleRequests()
}
