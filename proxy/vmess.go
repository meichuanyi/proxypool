package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/zu1k/proxypool/tool"
)

var (
	ErrorNotVmessLink          = errors.New("not a correct vmess link")
	ErrorVmessPayloadParseFail = errors.New("vmess link payload parse failed")
	//ErrorPasswordParseFail      = errors.New("password parse failed")
	//ErrorPathNotComplete        = errors.New("path not complete")
	//ErrorMissingQuery           = errors.New("link missing query")
	//ErrorProtocolParamParseFail = errors.New("protocol param parse failed")
	//ErrorObfsParamParseFail     = errors.New("obfs param parse failed")
)

type Vmess struct {
	Base
	UUID           string            `yaml:"uuid" json:"uuid"`
	AlterID        int               `yaml:"alterId" json:"alterId"`
	Cipher         string            `yaml:"cipher" json:"cipher"`
	TLS            bool              `yaml:"tls,omitempty" json:"tls,omitempty"`
	Network        string            `yaml:"network,omitempty" json:"network,omitempty"`
	HTTPOpts       HTTPOptions       `yaml:"http-opts,omitempty" json:"http-opts,omitempty"`
	WSPath         string            `yaml:"ws-path,omitempty" json:"ws-path,omitempty"`
	WSHeaders      map[string]string `yaml:"ws-headers,omitempty" json:"ws-headers,omitempty"`
	SkipCertVerify bool              `yaml:"skip-cert-verify,omitempty" json:"skip-cert-verify,omitempty"`
	ServerName     string            `yaml:"servername,omitempty" json:"servername,omitempty"`
}

type HTTPOptions struct {
	Method  string              `yaml:"method,omitempty" json:"method,omitempty"`
	Path    []string            `yaml:"path,omitempty" json:"path,omitempty"`
	Headers map[string][]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

func (v Vmess) Identifier() string {
	return net.JoinHostPort(v.Server, strconv.Itoa(v.Port)) + v.Cipher
}

func (v Vmess) String() string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

func (v Vmess) ToClash() string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return "- " + string(data)
}

type vmessLinkJson struct {
	Add  string      `json:"add"`
	V    string      `json:"v"`
	Ps   string      `json:"ps"`
	Port interface{} `json:"port"`
	Id   string      `json:"id"`
	Aid  string      `json:"aid"`
	Net  string      `json:"net"`
	Type string      `json:"type"`
	Host string      `json:"host"`
	Path string      `json:"path"`
	Tls  string      `json:"tls"`
}

func ParseVmessLink(link string) (Vmess, error) {
	if !strings.HasPrefix(link, "vmess") {
		return Vmess{}, ErrorNotVmessLink
	}

	vmessmix := strings.SplitN(link, "://", 2)
	if len(vmessmix) < 2 {
		return Vmess{}, ErrorNotVmessLink
	}
	linkPayload := vmessmix[1]
	if strings.Contains(linkPayload, "?") {
		// 使用第二种解析方法
		var infoPayloads []string
		if strings.Contains(linkPayload, "/?") {
			infoPayloads = strings.SplitN(linkPayload, "/?", 2)
		} else {
			infoPayloads = strings.SplitN(linkPayload, "?", 2)
		}
		if len(infoPayloads) < 2 {
			return Vmess{}, ErrorNotVmessLink
		}

		baseInfo, err := tool.Base64DecodeString(infoPayloads[0])
		if err != nil {
			return Vmess{}, ErrorVmessPayloadParseFail
		}
		fmt.Println(baseInfo)
		baseInfoPath := strings.Split(baseInfo, ":")
		if len(baseInfoPath) < 3 {
			return Vmess{}, ErrorPathNotComplete
		}
		// base info
		cipher := baseInfoPath[0]
		mixInfo := strings.SplitN(baseInfoPath[1], "@", 2)
		if len(mixInfo) < 2 {
			return Vmess{}, ErrorVmessPayloadParseFail
		}
		uuid := mixInfo[0]
		server := mixInfo[1]
		portStr := baseInfoPath[2]
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return Vmess{}, ErrorVmessPayloadParseFail
		}

		moreInfo, _ := url.ParseQuery(infoPayloads[1])
		fmt.Println(moreInfo)
		remarks := moreInfo.Get("remarks")
		obfs := moreInfo.Get("obfs")
		network := "tcp"
		if obfs == "websocket" {
			network = "ws"
		}
		//obfsParam := moreInfo.Get("obfsParam")
		path := moreInfo.Get("path")
		tls := moreInfo.Get("tls") == "1"

		wsHeaders := make(map[string]string)
		return Vmess{
			Base: Base{
				Name:   remarks + "_" + strconv.Itoa(rand.Int()),
				Server: server,
				Port:   port,
				Type:   "vmess",
				UDP:    false,
			},
			UUID:           uuid,
			AlterID:        0,
			Cipher:         cipher,
			TLS:            tls,
			Network:        network,
			HTTPOpts:       HTTPOptions{},
			WSPath:         path,
			WSHeaders:      wsHeaders,
			SkipCertVerify: true,
			ServerName:     server,
		}, nil
	} else {
		payload, err := tool.Base64DecodeString(linkPayload)
		if err != nil {
			return Vmess{}, ErrorVmessPayloadParseFail
		}
		vmessJson := vmessLinkJson{}
		err = json.Unmarshal([]byte(payload), &vmessJson)
		if err != nil {
			return Vmess{}, err
		}
		port := 443
		portInterface := vmessJson.Port
		if i, ok := portInterface.(int); ok {
			port = i
		} else if s, ok := portInterface.(string); ok {
			port, _ = strconv.Atoi(s)
		}

		alterId, err := strconv.Atoi(vmessJson.Aid)
		if err != nil {
			alterId = 0
		}
		tls := vmessJson.Tls == "tls"

		wsHeaders := make(map[string]string)
		wsHeaders["HOST"] = vmessJson.Host

		return Vmess{
			Base: Base{
				Name:   vmessJson.Ps + "_" + strconv.Itoa(rand.Int()),
				Server: vmessJson.Add,
				Port:   port,
				Type:   "vmess",
				UDP:    false,
			},
			UUID:           vmessJson.Id,
			AlterID:        alterId,
			Cipher:         "auto",
			TLS:            tls,
			Network:        vmessJson.Net,
			HTTPOpts:       HTTPOptions{},
			WSPath:         vmessJson.Path,
			WSHeaders:      wsHeaders,
			SkipCertVerify: true,
			ServerName:     vmessJson.Host,
		}, nil
	}
}

var (
	vmessPlainRe = regexp.MustCompile("vmess://([A-Za-z0-9+/_-])+")
)

func GrepVmessLinkFromString(text string) []string {
	results := make([]string, 0)
	texts := strings.Split(text, "vmess://")
	for _, text := range texts {
		results = append(results, vmessPlainRe.FindAllString("vmess://"+text, -1)...)
	}
	return results
}
