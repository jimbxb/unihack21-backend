package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"encoding/json"
	"github.com/gorilla/mux"
	"io/ioutil"
	"pkg/mod/github.com/google/uuid@v1.1.2"
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
	IP net.IPAddr `json:"serverId"`
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

func uploadModelHandler(w http.ResponseWriter, r *http.Request) {
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
