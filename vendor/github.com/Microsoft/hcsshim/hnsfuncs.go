package hcsshim

import (
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
)

func hnsCall(method, path, request string, returnResponse interface{}) error {
	var responseBuffer *uint16
	logrus.Debugf("[%s]=>[%s] Request : %s", method, path, request)

	err := _hnsCall(method, path, request, &responseBuffer)
	logrus.Debugf("HNS: [%v]", err)
	if err != nil {
		return makeError(err, "hnsCall ", "")
	}
	response := convertAndFreeCoTaskMemString(responseBuffer)
	logrus.Debugf("HNS: response:%v", response)
	hnsresponse := &hnsResponse{}
	if err = json.Unmarshal([]byte(response), &hnsresponse); err != nil {
		logrus.Debugf("HNS: failed to unmarshall err:%v", err)
		return err
	}

	if !hnsresponse.Success {
		return fmt.Errorf("HNS failed with error : %s", hnsresponse.Error)
	}

	if len(hnsresponse.Output) == 0 {
		logrus.Debugf("HNS: response.output == 0")
		return nil
	}

	logrus.Debugf("Network Response : %s", hnsresponse.Output)
	err = json.Unmarshal(hnsresponse.Output, returnResponse)
	if err != nil {
		return err
	}

	return nil
}
