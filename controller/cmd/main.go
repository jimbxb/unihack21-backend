package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/foomo/simplecert"
	"github.com/foomo/tlsconfig"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"io"
	"io/ioutil"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
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

type IOParams struct {
	InputFeatures  []ModelFeatures`json:"input_features"`
	OutputFeatures []ModelFeatures`json:"output_features"`
}

type ModelMetaData struct {
	ID             int32         `json:"id"`
	Name           string        `json:"name"`
	Desc           string        `json:"description"`
	IOParams       IOParams      `json:"io_params"`
}

type HostMetaData struct {
	BFS        string `json:"serverId"`
	ModelCount int32  `json:"modelCount"`
}

type NotifyDone struct {
	ID int32 `json:"id"`
}


func createModelHandler(w http.ResponseWriter, r *http.Request) {
	var model ModelMetaData



	model.ID = ModelCounter
	ModelCounter++
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
	if best == int32(math.MaxInt32) {
		return nil, fmt.Errorf("no hosts to get allocate to\n")
	} else {
		return res, nil
	}
}

func uploadModelHandler(w http.ResponseWriter, r *http.Request) {
	req, data, err := forwardModel(r)

	// check format

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(HostNotFound{
			Message: fmt.Sprintf("error in attempting to create request: %v\n", err),
			Host:    req.URL.String(),
		})
		log.Printf("error sending request: %v\n", err)
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
		log.Printf("error sending request: %v\n", err)
		return
	}

	res, _ := ioutil.ReadAll(ret.Body)


	log.Printf("response for model upload: %s, error: %v\n", string(res), err)

	if err == nil {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(data.ID)
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

	if val, ok := HostMap[data.ID]; ok {
		log.Println(fmt.Sprintf("found existing host for model: %d, %v\n", data.ID, host))
		return val.BFS, nil
	}

	host.ModelCount++
	HostMap[data.ID] = host
	log.Println(fmt.Sprintf("assigned data.modelID: %d to %v\n", data.ID, host))
	return host.BFS, nil
}


func forwardModel(r *http.Request) (*http.Request, *ModelMetaData, error) {
	_ = r.ParseMultipartForm(1<<31 - 1)

	// get the file out of the request
	file, header, err := r.FormFile("model")
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	// create a new buffer to write the multipart form data into
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	// create the model part
	part, err := writer.CreateFormFile("model", header.Filename)
	if err != nil {
		return nil, nil, err
	}
	_, _ = io.Copy(part, file)
	log.Printf("wrote file into part\n")

	name := r.FormValue("name")

	data := ModelMetaData{
		ID:       ModelCounter,
		Name:     name,
		Desc:     "A very useful model in training!",
	}
	ModelCounter++
	ModelMap[data.ID] = data

	// create the metadata part
	part, err = writer.CreateFormFile("metadata", "metadata.json")
	metadataFile, _ := json.MarshalIndent(data, "", "")
	tmpfile, _ := ioutil.TempFile(os.TempDir(), "tmp-")
	tmpfile.Write(metadataFile)
	defer tmpfile.Close()
	_, _ = io.Copy(part, tmpfile)

	// create the io_params part
	part, err = writer.CreateFormFile("io_params", "io_params.json")
	ioParamfile, _ := json.MarshalIndent(data.IOParams, "", "")
	tmpfile2, _ := ioutil.TempFile(os.TempDir(), "tmp2-")
	tmpfile2.Write(ioParamfile)
	defer tmpfile2.Close()
	_, _ = io.Copy(part, tmpfile2)

	err = writer.Close()
	if err != nil {
		return nil, nil, err
	}

	hosturl, err := assignModelHost(&data)
	if err != nil {
		return nil, nil, err
	}
	uri := fmt.Sprintf("%s/load/%d", hosturl, data.ID)
	Status[data.ID] = false
	request, _ := http.NewRequest("POST", uri, body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request, &data, nil
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

	res, _ := ioutil.ReadAll(ret.Body)
	if err == nil {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(string(res))
		log.Printf("processed eval model: %v\n", string(res))
	} else {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(HostNotFound{
			Message: fmt.Sprintf("no response received from requested host, status %v\n", err.Error()),
			Host:    req.URL.String(),
		})
		log.Printf("unsuccessful in processing eval model %v\n", err)
	}
}
func trainModelHandler(w http.ResponseWriter, r *http.Request) {

	_ = r.ParseMultipartForm(1<<31 - 1)

	// create a new buffer to write the multipart form data into
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	// get the training_data out of the request
	file, header, _ := r.FormFile("training_data")
	log.Printf("reading %s, size: %v\n", header.Filename, header.Size)
	defer file.Close()

	// create the training_data part
	part, _ := writer.CreateFormFile("training_data", header.Filename)
	_, _ = io.Copy(part, file)

	var params IOParams

	// get file out of the request
	file, header, _ = r.FormFile("io_params")
	log.Printf("reading %s, size: %v\n", header.Filename, header.Size)


	part, _ = writer.CreateFormFile("io_params", header.Filename)
	_, _ = io.Copy(part, file)

	f, _ := header.Open()
	defer f.Close()

	// write the IO params to io_params object
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, f); err != nil {
		log.Panic("Error writing io_params into buffer: \n\n", err)
	}

	log.Printf("length of buffer is %v\n", buf.Len())



	byt, _ := ioutil.ReadAll(buf)
	log.Printf("data inside ioparams: %v\n", string(byt))
	if err := json.Unmarshal(byt, &params); err != nil {
		log.Panic("Error unmarshalling ioparams\n\n", err)
	}

	name := r.FormValue("name")

	newModel := ModelMetaData{
		ID:       ModelCounter,
		Name:     name,
		Desc:     "A very useful model in training!",
		IOParams: params,
	}
	ModelCounter+=1
	hosturl, _ := assignModelHost(&newModel)

	ModelMap[newModel.ID] = newModel


	writer.Close()

	uri := fmt.Sprintf("%s/train/%d", hosturl, newModel.ID)
	req, _ := http.NewRequest("POST", uri, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())


	client := &http.Client{}
	ret, err := client.Do(req)

	res, _ := ioutil.ReadAll(ret.Body)
	if err == nil {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(newModel.ID)
		log.Printf("Successfully processed train model, %v\n", string(res))
	} else {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(HostNotFound{
			Message: fmt.Sprintf("no response received from requested host, status %v\n", err.Error()),
			Host:    req.URL.String(),
		})
		log.Printf("unsuccessful in processing train model %v\n", err)
	}
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

func getNodesHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(Hosts)
}

func handleRequests() {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/model", getModelsHandler).Methods("GET")
	router.HandleFunc("/model", uploadModelHandler).Methods("POST")
	router.HandleFunc("/eval/{id}", evalModelHandler).Methods("POST")
	router.HandleFunc("/train", trainModelHandler).Methods("POST")
	router.HandleFunc("/nodes", getNodesHandler).Methods("GET")
	handler := cors.AllowAll().Handler(router)
	s := &http.Server{
		Addr:      ":443",
		TLSConfig: setupDNS(),
		Handler: handler,
	}
	log.Fatal(s.ListenAndServeTLS("", ""))
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
		log.Printf("Added Host %v\n", v)
	}
}

func setupDNS() *tls.Config {

	// do the cert magic
	cfg := simplecert.Default
	cfg.Domains = []string{"api.kvoli.com"}
	cfg.CacheDir = "letsencrypt/live/api.kvoli.com"
	cfg.SSLEmail = "amcclernon@student.unimelb.edu.au"
	certReloader, err := simplecert.Init(cfg, nil)
	if err != nil {
		log.Fatal("simplecert init failed: ", err)
	}

	// redirect HTTP to HTTPS
	// CAUTION: This has to be done AFTER simplecert setup
	// Otherwise Port 80 will be blocked and cert registration fails!
	log.Println("starting HTTP Listener on Port 80")
	go http.ListenAndServe(":80", http.HandlerFunc(simplecert.Redirect))

	// init strict tlsConfig with certReloader
	// you could also use a default &tls.Config{}, but be warned this is highly insecure
	tlsconf := tlsconfig.NewServerTLSConfig(tlsconfig.TLSModeServerStrict)

	// now set GetCertificate to the reloaders GetCertificateFunc to enable hot reload
	tlsconf.GetCertificate = certReloader.GetCertificateFunc()
	return tlsconf
}

func main() {
	fmt.Println("listening on port 5000")
	makeInit()
	handleRequests()
}