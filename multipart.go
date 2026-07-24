package reqx

import (
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
)

type MultipartFormBuilder struct {
	requestBuilder *RequestBuilder
	formData       *MultipartFormData
}

func (m *MultipartFormBuilder) AddField(name, value string) *MultipartFormBuilder {
	m.formData.Fields = append(m.formData.Fields, FormField{
		Name:  name,
		Value: value,
	})
	return m
}

func (m *MultipartFormBuilder) AddFile(fieldName, fileName string, data []byte) *MultipartFormBuilder {
	m.formData.Files = append(m.formData.Files, FormFile{
		FieldName: fieldName,
		FileName:  fileName,
		Data:      data,
	})
	return m
}

func (m *MultipartFormBuilder) AddFileReader(fieldName, fileName string, reader io.Reader) *MultipartFormBuilder {
	m.formData.Files = append(m.formData.Files, FormFile{
		FieldName: fieldName,
		FileName:  fileName,
		Reader:    reader,
	})
	return m
}

func (m *MultipartFormBuilder) Do(successTarget any, errorTarget any) (*Response, error) {
	m.requestBuilder.contentType = ContentTypeMultipartForm
	m.requestBuilder.body = m.formData
	return m.requestBuilder.Do(successTarget, errorTarget)
}

func multipartBody(formData *MultipartFormData) (*bodySource, error) {
	for _, file := range formData.Files {
		if file.Reader != nil {
			reader, contentType := streamMultipartForm(formData)
			return singleUseBody(reader, contentType), nil
		}
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writeMultipartForm(writer, formData); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	return replayableBody(buf.Bytes(), writer.FormDataContentType()), nil
}

func streamMultipartForm(formData *MultipartFormData) (io.Reader, string) {
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	contentType := writer.FormDataContentType()

	go func() {
		writeErr := writeMultipartForm(writer, formData)
		if writeErr == nil {
			writeErr = writer.Close()
		}

		var err error
		if writeErr != nil {
			err = pipeWriter.CloseWithError(writeErr)
		} else {
			err = pipeWriter.Close()
		}

		if err != nil {
			slog.Error("failed to close pipe writer",
				"component", "streamMultipartForm",
				"error", err)
		}
	}()

	return pipeReader, contentType
}

func writeMultipartForm(writer *multipart.Writer, formData *MultipartFormData) error {
	for _, field := range formData.Fields {
		if err := writer.WriteField(field.Name, field.Value); err != nil {
			return err
		}
	}

	for _, file := range formData.Files {
		part, err := writer.CreateFormFile(file.FieldName, file.FileName)
		if err != nil {
			return err
		}

		switch {
		case file.Reader != nil:
			if _, err := io.Copy(part, file.Reader); err != nil {
				return err
			}
		case file.Data != nil:
			if _, err := part.Write(file.Data); err != nil {
				return err
			}
		}
	}

	return nil
}
