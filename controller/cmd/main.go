package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

var ModelMap = make(map[int32]ModelMetaData, 1)
var HostMap = make(map[int32]*HostMetaData, 1)
var Hosts = make([]*HostMetaData, 0)

var Status = make(map[int32]bool)

var ModelCounter int32 = 0

type ModelFeatures struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Encoder string `json:"encoder"`
}

type HostNotFound struct {
	Message string `json:"message"`
	Host    string `json:"host"`
}

type ModelMetaData struct {
	ID             int32         `json:"id"`
	Name           string        `json:"name"`
	Desc           string        `json:"description"`
	InputFeatures  []ModelFeatures `json:"input_features"`
	OutputFeatures []ModelFeatures `json:"output_features"`
}

type HostMetaData struct {
	BFS        string `json:"serverId"`
	ModelCount int32  `json:"modelCount"`
}

type NotifyDone struct {
	ID int32 `json:"id"`
}

func createModelHandler(w http.ResponseWriter, r *http.Request) {
	reqBody, _ := ioutil.ReadAll(r.Body)
	var model ModelMetaData
	_ = json.Unmarshal(reqBody, &model)

	model.ID = ModelCounter
	ModelCounter += 1
	ModelMap[model.ID] = model
	_ = json.NewEncoder(w).Encode(model.ID)
}

func getRequestID(r *http.Request) int32 {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])
	numId := int32(id)
	return numId
}

func getNextHost() (*HostMetaData, error) {
	best := int32(math.MaxInt32)
	var res *HostMetaData
	for _, host := range Hosts {
		if host.ModelCount < best {
			res = host
			best = host.ModelCount
		}
	}
	if (best == int32(math.MaxInt32)) {
		return nil, fmt.Errorf("no hosts to get allocate to\n")
	} else {
		return res, nil
	}
}

func uploadModelHandler(w http.ResponseWriter, r *http.Request) {
	reqId := getRequestID(r)
	data := ModelMap[reqId]

	var res map[string]interface{}

	req, err := forwardModel(&data, r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(HostNotFound{
			Message: fmt.Sprintf("error in attempting to create request: %v\n", err),
			Host:    req.URL.String(),
		})
		fmt.Printf("error sending request: %v\n", err)
		return
	}

	client := &http.Client{}
	ret, err := client.Do(req)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(HostNotFound{
			Message: fmt.Sprintf("error in request to server %v\n", err),
			Host:    req.URL.String(),
		})
		fmt.Printf("error sending request: %v\n", err)
		return
	}

	json.NewDecoder(ret.Body).Decode(res)

	fmt.Printf("")
	fmt.Printf("response for model upload: %v, error: %v\n", res, err)

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

func assignModelHost(data *ModelMetaData) (string, error){
	host, err := getNextHost()
	if err != nil {
		return "", err
	}

	HostMap[data.ID] = host
	host.ModelCount += 1
	fmt.Println(fmt.Sprintf("assigned data.modelID: %d to %v\n", data.ID, host))
	return host.BFS, nil
}

func forwardModel(data *ModelMetaData, r *http.Request) (*http.Request, error) {
	_ = r.ParseMultipartForm(1<<31 - 1)
	_, header, err := r.FormFile("model")
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

	fmt.Printf("wrote file into part\n")

	metadataFile, _ := json.MarshalIndent(*data, "", "")
	part, err = writer.CreateFormFile("metadata", "metadata.json")

	tmpfile, _ := ioutil.TempFile(os.TempDir(), "tmp-")
	tmpfile.Write(metadataFile)
	defer tmpfile.Close()
	_, _ = io.Copy(part, tmpfile)

	part, err = writer.CreateFormFile("io_params", "io_params.json")
	_, _ = io.Copy(part, tmpfile)

	fmt.Printf("wrote json part\n")

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	hosturl, err := assignModelHost(data)
	if err != nil {
		return nil, err
	}
	uri := fmt.Sprintf("%s/load/%d", hosturl, data.ID)
	Status[data.ID] = false
	request, _ := http.NewRequest("POST", uri, body)
	return request, nil
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
		fmt.Printf("Successfully processed eval model\n")
	} else {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(HostNotFound{
			Message: fmt.Sprintf("no response received from requested host, status %v\n", err.Error()),
			Host:    req.URL.String(),
		})
		fmt.Printf("unsuccessful in processing eval model %v\n", err)
	}
}
func trainModelHandler(w http.ResponseWriter, r *http.Request) {

}

func onFinished(w http.ResponseWriter, r *http.Request) {
	var nd NotifyDone

	err := json.NewDecoder(r.Body).Decode(&nd)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	Status[nd.ID] = true
	return
}

func handleRequests() {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/model", getModelsHandler).Methods("GET")
	router.HandleFunc("/model", createModelHandler).Methods("POST")
	router.HandleFunc("/model/{id}", uploadModelHandler).Methods("POST")
	router.HandleFunc("/eval/{id}", evalModelHandler).Methods("POST")
	router.HandleFunc("/train/{id}", trainModelHandler).Methods("POST")
	handler := cors.AllowAll().Handler(router)
	log.Fatal(http.ListenAndServe(":5000", handler))
}

func makeInit() {
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
