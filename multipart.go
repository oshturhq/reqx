package httpclient

import (
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

func (b *RequestBuilder) buildMultipartForm(formData *MultipartFormData) (io.Reader, string, error) {
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	contentType := writer.FormDataContentType()

	go func() {
		var writeErr error
		defer func() {
			if writeErr != nil {
				err := pipeWriter.CloseWithError(writeErr)
				if err != nil {
					slog.Error("Failed to close pipe writer",
						"component", "buildMultipartForm",
						"error", err)
				}
			} else {
				err := pipeWriter.Close()
				if err != nil {
					slog.Error("Failed to close pipe writer",
						"component", "buildMultipartForm",
						"error", err)
				}
			}
		}()

		for _, field := range formData.Fields {
			writeErr = writer.WriteField(field.Name, field.Value)
			if writeErr != nil {
				return
			}
		}

		for _, file := range formData.Files {
			part, err := writer.CreateFormFile(file.FieldName, file.FileName)
			if err != nil {
				writeErr = err
				return
			}

			if file.Reader != nil {
				_, writeErr = io.Copy(part, file.Reader)
			} else if file.Data != nil {
				_, writeErr = part.Write(file.Data)
			}

			if writeErr != nil {
				return
			}
		}

		writeErr = writer.Close()
	}()

	return pipeReader, contentType, nil
}
