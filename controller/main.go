package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"pkg/mod/github.com/google/uuid@v1.1.2"
	"reflect"
)

var ModelMap map[uuid.UUID]ModelMetaData
var HostMap map[uuid.UUID]HostMetaData
var Hosts []HostMetaData



type ModelFeatures struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Encoder string `json:"encoder"`
}

type ModelMetaData struct {
	ID      uuid.UUID `json:"id"`
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

	model.ID = uuid.New()
	ModelMap[model.ID] = model
	_ = json.NewEncoder(w).Encode(model.ID)
}

func getRequestID(r *http.Request) uuid.UUID {
	vars := mux.Vars(r)
	id, _ :=  uuid.Parse(vars["id"])
	return id
}

func getNextHost() HostMetaData {
	res := Hosts[0]
	for _, host := range Hosts {
		if host.ModelCount < res.ModelCount {
			res = host
		}
	}
	return res
}

func uploadModelHandler(w http.ResponseWriter, r *http.Request) {
	reqId := getRequestID(r)
	data := ModelMap[reqId]

	req, _ := forwardModel(&data, r)
	client := &http.Client{}
	client.Do(req)
}


func forwardModel(data *ModelMetaData, r *http.Request) (*http.Request, error) {
	r.ParseMultipartForm(10 << 20)
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
	io.Copy(part, file)

	v := reflect.ValueOf(data)
	typeOfData := v.Type()

	for i := 0; i < v.NumField(); i++ {
		writer.WriteField(typeOfData.Field(i).Name, fmt.Sprint(v.Field(i).Interface()))
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}
	return http.NewRequest("POST", fmt.Sprintf(getNextHost().BFS + "/model"), body)
}

func getModelsHandler(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(ModelMap)
}

func evalModelHandler(w http.ResponseWriter, r *http.Request) {
}

func handleRequests() {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/model", getModelsHandler).Methods("GET")
	router.HandleFunc("/model", createModelHandler).Methods("POST")
	router.HandleFunc("/model/{id}", uploadModelHandler).Methods("POST")
	router.HandleFunc("/eval/{id}", evalModelHandler).Methods("POST")
	log.Fatal(http.ListenAndServe(":8080", router))
}


func makeInit(){
}


func main() {
	fmt.Println("listening on port 8080")
	makeInit()
	handleRequests()
}
