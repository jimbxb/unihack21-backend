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
	"reflect"
	"strconv"
)

var ModelMap = make(map[int32]ModelMetaData, 1)
var HostMap = make(map[int32]HostMetaData, 1)
var Hosts = make ([]HostMetaData, 1)

var ModelCounter int32 = 0


type ModelFeatures struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Encoder string `json:"encoder"`
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

	v := reflect.ValueOf(*data)
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