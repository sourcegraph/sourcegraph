package graphql

import (
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/resolvers/dataloader"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
)

type (
	UploadLoaderFactory = *dataloader.LoaderFactory[int, shared.Upload]
	IndexLoaderFactory  = *dataloader.LoaderFactory[int, shared.AutoIndexJob]
	UploadLoader        = *dataloader.Loader[int, shared.Upload]
	IndexLoader         = *dataloader.Loader[int, shared.AutoIndexJob]
)

func NewUploadLoaderFactory(uploadService UploadsService) UploadLoaderFactory {
	return dataloader.NewLoaderFactory[int, shared.Upload](dataloader.BackingServiceFunc[int, shared.Upload](uploadService.GetUploadsByIDs))
}

func NewIndexLoaderFactory(uploadService UploadsService) IndexLoaderFactory {
	return dataloader.NewLoaderFactory[int, shared.AutoIndexJob](dataloader.BackingServiceFunc[int, shared.AutoIndexJob](uploadService.GetAutoIndexJobsByIDs))
}

func PresubmitAssociatedIndexes(indexLoader IndexLoader, uploads ...shared.Upload) {
	for _, upload := range uploads {
		if upload.AssociatedIndexID != nil {
			indexLoader.Presubmit(*upload.AssociatedIndexID)
		}
	}
}

func PresubmitAssociatedUploads(uploadLoader UploadLoader, jobs ...shared.AutoIndexJob) {
	for _, job := range jobs {
		if job.AssociatedUploadID != nil {
			uploadLoader.Presubmit(*job.AssociatedUploadID)
		}
	}
}
