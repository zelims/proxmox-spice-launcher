package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"net/http"
	"net/url"
	"io/ioutil"
	"strings"
	"time"
	"os/exec"
	"path/filepath"
	"encoding/json"
	"crypto/tls"
	"github.com/tidwall/gjson"
)

var config Config
var spiceConfig Spice

type Config struct {
	ID 				int
	Username 			string
	Password 			string
	Node 				string
	Host 				string
	Ticket 				string
	CSRF 				string
	ViewerPath 			string
}

type Spice struct {
	Attention 			string 	`json:"secure-attention"`
	Delete				int 	`json:"delete-this-file"`
	Proxy				string 	`json:"proxy"`
	Type				string 	`json:"type"`
	CA				string 	`json:"ca"`
	Fullscreen			string 	`json:"toggle-fullscreen"`
	Title				string 	`json:"title"`
	Host				string 	`json:"host"`
	Password			string 	`json:"password"`
	Subject				string 	`json:"host-subject"`
	Cursor				string 	`json:"release-cursor"`
	Port				int 	`json:"tls-port""`
}

func init() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	if len(os.Args) < 2 {
		fmt.Println("error(init.args): you must supply a VM ID")
		os.Exit(1)
	}

	id, err := strconv.Atoi(os.Args[1])
	hasError("init", "vm id must be an integer", err)

	if id == -1 || id < 100 {
		fmt.Println("error(init.check): invalid vmid")
		os.Exit(1)
	}

	config.ID = id
}

func hasError(section string, message string, err error) {
	if err != nil {
		if message == "" {
			fmt.Println(fmt.Sprintf("error(%s):", section), err.Error())
		} else {
			fmt.Println(fmt.Sprintf("error(%s): %s", section, message))
		}
		os.Exit(1)
	}
}

func readConfig(configFile string) ([]string, error) {
	file, err := os.Open(configFile)
	hasError("fileOpen", "", err)
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func Authenticate() bool {
	data := url.Values{}
	data.Set("username", config.Username)
	data.Set("password", config.Password)

	request, err := http.NewRequest("POST", 
		fmt.Sprintf("https://%s:8006/api2/json/access/ticket", config.Host),
			strings.NewReader(data.Encode()))
	hasError("auth.request", "", err)

	body := doRequest(request)

	ticket := gjson.Get(string(body), "data.ticket")
	csrf := gjson.Get(string(body), "data.CSRFPreventionToken")
	if !csrf.Exists() || !ticket.Exists() {
		return false
	}

	config.Ticket = ticket.String()
	config.CSRF = csrf.String()
	return true
}

func doRequest(request *http.Request) []byte {
	client := &http.Client{}
	resp, err := client.Do(request)
	hasError("do.response", "", err)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	
	return body
}

func BuildRequest(reqType, endpoint string, query map[string]string) []byte {
	data := url.Values{}
	if query != nil {
		for key, value := range query {
			data.Set(key, value)
		}
	}
	url := fmt.Sprintf("https://%s:8006/api2/json/nodes/%s/qemu/%d%s", config.Host, config.Node, config.ID, endpoint)
	request, err := http.NewRequest(reqType, url, strings.NewReader(data.Encode()))
	request.Header.Set("CSRFPreventionToken", config.CSRF)
	request.AddCookie(&http.Cookie{ Name: "PVEAuthCookie", Value: config.Ticket, HttpOnly: false })
	hasError("build.request", "", err)
	
	return doRequest(request)
}

func main() {
	ex, _ := os.Executable()
	cfg, err := readConfig(fmt.Sprintf("%s/%s", filepath.Dir(ex), ".pve"))
	hasError("readFile", "", err)

	config.Node = cfg[0]
	config.Host = cfg[1]
	config.Username = cfg[2]
	config.Password = cfg[3] 
	config.ViewerPath = cfg[4]

	if !Authenticate() {
		fmt.Println("error(auth): could not authenticate")
		os.Exit(1)
	}

	status := gjson.Get(string(BuildRequest("GET", "/status/current", nil)), "data.qmpstatus")
	if !status.Exists() {
		fmt.Println("error(status): could not get current status of", config.ID)
		os.Exit(1)
	}

	if status.String() == "stopped" {
		fmt.Printf("ERROR: VM %d is not running. Attempting to start it", config.ID)
		BuildRequest("GET", "/status/start", nil)
		for i := 0; i < 15; i++ {
			fmt.Printf(".")
			time.Sleep(250 * time.Millisecond)
		}
		fmt.Printf("\n")
	}

	spice := BuildRequest("POST", "/spiceproxy", map[string]string {
		"proxy": config.Host,
	})
	
	err = json.Unmarshal([]byte(gjson.Get(string(spice), "data").String()), &spiceConfig)
	hasError("spice", "", err)

	viewerProcess := exec.Command(config.ViewerPath, "-")
	stdin, err := viewerProcess.StdinPipe()
	hasError("exec", "", err)
	defer stdin.Close()

	err = viewerProcess.Start()
	hasError("start", "", err)

	_, err = fmt.Fprintf(stdin, "[virt-viewer]\n"+
		"tls-port=%d\n"+
		"delete-this-file=%d\n"+
		"title=%s\n"+
		"proxy=%s\n"+
		"toggle-fullscreen=%s\n"+
		"type=%s\n"+
		"host-subject=%s\n"+
		"release-cursor=%s\n"+ 
		"password=%s\n"+
		"secure-attention=%s\n"+
		"host=%s\n"+
		"ca=%s\n",
		spiceConfig.Port, spiceConfig.Delete, spiceConfig.Title, spiceConfig.Proxy, spiceConfig.Fullscreen,
		spiceConfig.Type, spiceConfig.Subject, spiceConfig.Cursor, spiceConfig.Password, spiceConfig.Attention,
		spiceConfig.Host, spiceConfig.CA)

	hasError("stdin", "", err)

}
