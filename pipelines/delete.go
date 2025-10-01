package pipelines

import (
	"context"
	"errors"
	"fmt"

	"github.com/nixmade/pippy/store"
)

func DeletePipeline(ctx context.Context, name string) error {
	// confirm here that users intention was to delete
	dbStore, err := store.Get(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := store.Close(dbStore); closeErr != nil {
			err = closeErr
		}
	}()

	if err := dbStore.Delete(PipelinePrefix + name); err != nil {
		if errors.Is(err, store.ErrKeyNotFound) {
			return nil
		}
		return err
	}

	if err := dbStore.DeletePrefix(fmt.Sprintf("%s%s/", PipelineRunPrefix, name)); err != nil {
		return err
	}

	return nil
}

func DeletePipelineRun(ctx context.Context, name, id string) error {
	// confirm here that users intention was to delete
	dbStore, err := store.Get(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := store.Close(dbStore); closeErr != nil {
			err = closeErr
		}
	}()

	pipelineRunKey := fmt.Sprintf("%s%s/%s", PipelineRunPrefix, name, id)
	if err := dbStore.Delete(pipelineRunKey); err != nil {
		if errors.Is(err, store.ErrKeyNotFound) {
			return nil
		}
		return err
	}

	return nil
}
