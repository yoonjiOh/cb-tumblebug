package mcis

import (

	//"encoding/json"
	"fmt"
	"io/ioutil"

	//"log"

	//"strings"
	"strconv"

	"bytes"
	"mime/multipart"

	// REST API (echo)
	"net/http"

	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"

)


type MonAgentInstallReq struct {
	Mcis_id string `json:"mcis_id"`
	Vm_id   string `json:"vm_id"`
	Public_ip   string `json:"public_ip"`
	User_name  string `json:"user_name"`
	Ssh_key  string `json:"ssh_key"`
}


func CallMonitoringAsync(wg *sync.WaitGroup, mcisID string, vmID string, vmIP string, userName string, privateKey string, method string, cmd string, returnResult *[]SshCmdResult) {

	defer wg.Done() //goroutin sync done

	url := common.DRAGONFLY_REST_URL + cmd
	fmt.Println("\n\n[Calling DRAGONFLY] START")
	fmt.Println("url: " + url + " method: " + method)

	tempReq := MonAgentInstallReq{
		Mcis_id: mcisID,
		Vm_id: vmID,
		Public_ip: vmIP,
		User_name: userName,
		Ssh_key: privateKey,
	}
	fmt.Printf("\n[Request body to CB-DRAGONFLY for installing monitoring agent in VM]\n")
	common.PrintJsonPretty(tempReq)

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	_ = writer.WriteField("mcis_id", mcisID)
	_ = writer.WriteField("vm_id", vmID)
	_ = writer.WriteField("public_ip", vmIP)
	_ = writer.WriteField("user_name", userName)
	_ = writer.WriteField("ssh_key", privateKey)
	err := writer.Close()

	errStr := ""
	if err != nil {
	  common.CBLog.Error(err)
	  errStr = err.Error()
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, payload)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := client.Do(req)

	fmt.Println("Called CB-DRAGONFLY API")
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		common.CBLog.Error(err)
		errStr = err.Error()
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		common.CBLog.Error(err)
		errStr = err.Error()
	}

	result := string(body)

	//wg.Done() //goroutin sync done

	sshResultTmp := SshCmdResult{}
	sshResultTmp.Mcis_id = mcisID
	sshResultTmp.Vm_id = vmID
	sshResultTmp.Vm_ip = vmIP

	if err != nil {
		sshResultTmp.Result = errStr
		sshResultTmp.Err = err
		*returnResult = append(*returnResult, sshResultTmp)
	} else {
		fmt.Println("result " + result)
		sshResultTmp.Result = result
		sshResultTmp.Err = nil
		*returnResult = append(*returnResult, sshResultTmp)
	}

}


func InstallMonitorAgentToMcis(nsId string, mcisId string, req *McisCmdReq) (AgentInstallContentWrapper, error) {

	content := AgentInstallContentWrapper{}

	//install script
	cmd := "/agent/install"

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return content, err
	}

	//goroutin sync wg
	var wg sync.WaitGroup

	var resultArray []SshCmdResult

	method := "POST"

	for _, v := range vmList {
		wg.Add(1)

		vmId := v
		vmIp := GetVmIp(nsId, mcisId, vmId)

		//cmd := req.Command

		// userName, sshKey := GetVmSshKey(nsId, mcisId, vmId)
		// if (userName == "") {
		// 	userName = req.User_name
		// }
		// if (userName == "") {
		// 	userName = sshDefaultUserName
		// }

		// find vaild username
		userName, sshKey := GetVmSshKey(nsId, mcisId, vmId)
		userNames := []string{SshDefaultUserName01, SshDefaultUserName02, SshDefaultUserName03, SshDefaultUserName04, userName, req.User_name}
		userName = VerifySshUserName(vmIp, userNames, sshKey)

		fmt.Println("[SSH] " + mcisId + "/" + vmId + "(" + vmIp + ")" + "with userName:" + userName)
		fmt.Println("[CMD] " + cmd)

		go CallMonitoringAsync(&wg, mcisId, vmId, vmIp, userName, sshKey, method, cmd, &resultArray)

	}
	wg.Wait() //goroutin sync wg

	for _, v := range resultArray {

		resultTmp := AgentInstallContent{}
		resultTmp.Mcis_id = mcisId
		resultTmp.Vm_id = v.Vm_id
		resultTmp.Vm_ip = v.Vm_ip
		resultTmp.Result = v.Result
		content.Result_array = append(content.Result_array, resultTmp)
		//fmt.Println("result from goroutin " + v)
	}

	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return content, nil

}



func GetMonitoringData(nsId string, mcisId string, metric string) (AgentInstallContentWrapper, error) {

	content := AgentInstallContentWrapper{}

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return content, err
	}

	//goroutin sync wg
	var wg sync.WaitGroup

	var resultArray []SshCmdResult

	method := "GET"

	for _, v := range vmList {
		wg.Add(1)

		vmId := v

		cmd := "/mcis/"+mcisId+"/vm/"+vmId+"/metric/"+metric+"/rt-info?statisticsCriteria=avg"
		fmt.Println("[CMD] " + cmd)

		go CallGetMonitoringAsync(&wg, mcisId, vmId, method, cmd, &resultArray)

	}
	wg.Wait() //goroutin sync wg

	for _, v := range resultArray {

		resultTmp := AgentInstallContent{}
		resultTmp.Mcis_id = mcisId
		resultTmp.Vm_id = v.Vm_id
		resultTmp.Vm_ip = v.Vm_ip
		resultTmp.Result = v.Result
		content.Result_array = append(content.Result_array, resultTmp)
		//fmt.Println("result from goroutin " + v)
	}

	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return content, nil

}



func CallGetMonitoringAsync(wg *sync.WaitGroup, mcisID string, vmID string, method string, cmd string, returnResult *[]SshCmdResult) {

	defer wg.Done() //goroutin sync done

	url := common.DRAGONFLY_REST_URL + cmd
	fmt.Println("\n\n[Calling DRAGONFLY] START")
	fmt.Println("url: " + url + " method: " + method)

	tempReq := MonAgentInstallReq{
		Mcis_id: mcisID,
		Vm_id: vmID,
	}
	fmt.Printf("\n[Request body to CB-DRAGONFLY for installing monitoring agent in VM]\n")
	common.PrintJsonPretty(tempReq)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, nil)
	errStr := ""
	if err != nil {
		common.CBLog.Error(err)
		errStr = err.Error()
	}

	res, err := client.Do(req)

	fmt.Println("Called CB-DRAGONFLY API")
	if err != nil {
		common.CBLog.Error(err)
		errStr = err.Error()
	}
	
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		common.CBLog.Error(err)
		errStr = err.Error()
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		common.CBLog.Error(err)
		errStr = err.Error()
	}

	result := string(body)

	//wg.Done() //goroutin sync done

	sshResultTmp := SshCmdResult{}
	sshResultTmp.Mcis_id = mcisID
	sshResultTmp.Vm_id = vmID

	if err != nil {
		sshResultTmp.Result = errStr
		sshResultTmp.Err = err
		*returnResult = append(*returnResult, sshResultTmp)
	} else {
		fmt.Println("result " + result)
		sshResultTmp.Result = result
		sshResultTmp.Err = nil
		*returnResult = append(*returnResult, sshResultTmp)
	}

}