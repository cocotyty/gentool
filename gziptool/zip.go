package gziptool

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
)

func JSONGzip(obj interface{}) (data []byte, err error) {
	jsonData, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	buffer := bytes.NewBuffer(nil)
	writer := gzip.NewWriter(buffer)
	writer.Write(jsonData)
	writer.Close()
	return buffer.Bytes(), nil
}

func GUnzipJSON(data []byte, obj interface{}) (err error) {
	buffer := bytes.NewReader(data)
	reader, err := gzip.NewReader(buffer)
	if err != nil {
		return err
	}
	return json.NewDecoder(reader).Decode(obj)
}
