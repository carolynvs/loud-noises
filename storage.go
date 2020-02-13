package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest"
	"github.com/pkg/errors"
)

type Storage struct {
	Account    string
	credential azblob.Credential
	pipeline   pipeline.Pipeline
}

func NewStorageClient() (Storage, error) {
	s := Storage{Account: "slackoverload"}

	a, err := getAzureAuth(s.URL())
	if err != nil {
		return s, err
	}

	fakeAuthRequest := &http.Request{}
	fakeAuthRequest, err = autorest.Prepare(fakeAuthRequest, a.WithAuthorization())
	if err != nil {
		return s, errors.Wrap(err, "could not get auth token from authorizer")
	}
	authHeader := fakeAuthRequest.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	s.credential = azblob.NewTokenCredential(token, nil)
	s.pipeline = azblob.NewPipeline(s.credential, azblob.PipelineOptions{})

	return s, nil
}

func (s *Storage) URL() string {
	return fmt.Sprintf("https://%s.blob.core.windows.net", s.Account)
}

func (s *Storage) listContainer(containerName string, prefix string) ([]string, error) {
	container, err := s.buildContainerURL(containerName)
	if err != nil {
		return nil, err
	}

	var names []string
	for marker := (azblob.Marker{}); marker.NotDone(); {
		response, err := container.ListBlobsFlatSegment(context.Background(), marker, azblob.ListBlobsSegmentOptions{
			Prefix: prefix,
		})
		if err != nil {
			return nil, errors.Wrapf(err, "error listing container %s with prefix %s", containerName, prefix)
		}

		marker = response.NextMarker
		for _, blobInfo := range response.Segment.BlobItems {
			names = append(names, blobInfo.Name)
		}
	}

	return names, nil
}

func (s *Storage) getBlob(containerName string, blobName string) ([]byte, error) {
	containerURL, err := s.buildContainerURL(containerName)
	if err != nil {
		return nil, err
	}

	blobURL := containerURL.NewBlobURL(blobName)

	resp, err := blobURL.Download(context.Background(), 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false)
	if err != nil {
		return nil, errors.Wrapf(err, "error initiating download of blob at %s", blobURL.String())
	}

	bodyStream := resp.Body(azblob.RetryReaderOptions{MaxRetryRequests: 20})
	buff := bytes.Buffer{}
	_, err = buff.ReadFrom(bodyStream)

	return buff.Bytes(), errors.Wrapf(err, "error reading blob body at %s", blobURL.String())
}

func (s *Storage) setBlob(containerName string, blobName string, data []byte) error {
	container, err := s.buildContainerURL(containerName)
	if err != nil {
		return err
	}

	blob := container.NewBlockBlobURL(blobName)
	opts := azblob.UploadToBlockBlobOptions{BlockSize: 64 * 1024}

	_, err = azblob.UploadBufferToBlockBlob(context.Background(), data, blob, opts)
	return errors.Wrapf(err, "error saving %s/%s", containerName, blobName)
}

func (s *Storage) buildContainerURL(containerName string) (azblob.ContainerURL, error) {
	rawURL := fmt.Sprintf("%s/%s", s.URL(), containerName)
	URL, err := url.Parse(rawURL)
	if err != nil {
		return azblob.ContainerURL{}, errors.Wrapf(err, "could not parse container URL %s", rawURL)
	}

	return azblob.NewContainerURL(*URL, s.pipeline), nil
}
