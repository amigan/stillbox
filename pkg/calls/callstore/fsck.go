package callstore

import (
	"context"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/database"
)

type FsckParams struct {
	Backend string `json:"backend" desc:"backend to check" flag:"backend b"`

	ProgressChan chan FsckReport `json:"-"`
}

type FsckReport struct {
	Status             *string `json:"status,omitempty"`
	FinalCallsDangling *int64  `json:"finalCallsDangling,omitempty"`
	CallsEnumerated    *int64  `json:"callsEnumerated,omitempty"`
	Error              *string `json:"error,omitempty"`
}

func (s *store) Fsck(ctx context.Context, par FsckParams) (result FsckReport, err error) {
	if !s.maintInProgress.TryLock() {
		return FsckReport{}, ErrMaintenanceInProgress
	}
	defer s.maintInProgress.Unlock()

	err = s.db.InTx(ctx, func(tx database.Store) error {
		defer func() {
		}()
		backend := s.audioBackends.Backend(par.Backend)
		if backend == nil {
			return ErrNXBackend
		}

		var copyProgCh chan int64
		if par.ProgressChan != nil {
			copying := "enumerating calls in backend"
			copyProgCh = make(chan int64)
			go func() {
				for pr := range copyProgCh {
					par.ProgressChan <- FsckReport{
						Status:          &copying,
						CallsEnumerated: common.PtrTo(pr), // takes address of copy
					}
				}
			}()
		}

		tableName, err := tx.CreateFsckTempTable(ctx)
		if err != nil {
			return err
		}

		it, errf := backend.ListCalls(ctx, "")
		copied, err := tx.CopyIntoFsckTempTable(ctx, tableName, it, copyProgCh)
		if err != nil {
			return err
		}

		defer tx.DropTable(ctx, tableName)

		if err := errf(); err != nil {
			return err
		}

		if par.ProgressChan != nil {
			par.ProgressChan <- FsckReport{
				Status:          common.PtrTo("checking calls"),
				CallsEnumerated: &copied,
			}
		}

		callsDangling, err := tx.FsckRefs(ctx, tableName, par.Backend)
		if err != nil {
			return err
		}

		result.Status = common.PtrTo("finished")
		result.CallsEnumerated = &copied
		result.FinalCallsDangling = &callsDangling

		return nil
	})
	return
}
