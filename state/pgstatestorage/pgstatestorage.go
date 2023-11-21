package pgstatestorage

import (
	"context"
	"errors"

	"github.com/0xPolygonHermez/zkevm-sequencer/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// PostgresStorage implements the Storage interface
type PostgresStorage struct {
	cfg state.Config
	*pgxpool.Pool
}

// NewPostgresStorage creates a new StateDB
func NewPostgresStorage(cfg state.Config, db *pgxpool.Pool) *PostgresStorage {
	return &PostgresStorage{
		cfg,
		db,
	}
}

// getExecQuerier determines which execQuerier to use, dbTx or the main pgxpool
func (p *PostgresStorage) getExecQuerier(dbTx pgx.Tx) ExecQuerier {
	if dbTx != nil {
		return dbTx
	}
	return p
}

// Reset resets the state to a block for the given DB tx
func (p *PostgresStorage) Reset(ctx context.Context, blockNumber uint64, dbTx pgx.Tx) error {
	e := p.getExecQuerier(dbTx)
	const resetSQL = "DELETE FROM state.block WHERE block_num > $1"
	if _, err := e.Exec(ctx, resetSQL, blockNumber); err != nil {
		return err
	}

	return nil
}

// ResetForkID resets the state to reprocess the newer batches with the correct forkID
func (p *PostgresStorage) ResetForkID(ctx context.Context, batchNumber uint64, dbTx pgx.Tx) error {
	e := p.getExecQuerier(dbTx)
	const resetVirtualStateSQL = "delete from state.block where block_num >=(select min(block_num) from state.virtual_batch where batch_num >= $1)"
	if _, err := e.Exec(ctx, resetVirtualStateSQL, batchNumber); err != nil {
		return err
	}
	err := p.ResetTrustedState(ctx, batchNumber-1, dbTx)
	if err != nil {
		return err
	}

	// Delete proofs for higher batches
	const deleteProofsSQL = "delete from state.proof where batch_num >= $1 or (batch_num <= $1 and batch_num_final  >= $1)"
	if _, err := e.Exec(ctx, deleteProofsSQL, batchNumber); err != nil {
		return err
	}

	return nil
}

// ResetTrustedState removes the batches with number greater than the given one
// from the database.
func (p *PostgresStorage) ResetTrustedState(ctx context.Context, batchNumber uint64, dbTx pgx.Tx) error {
	const resetTrustedStateSQL = "DELETE FROM state.batch WHERE batch_num > $1"
	e := p.getExecQuerier(dbTx)
	if _, err := e.Exec(ctx, resetTrustedStateSQL, batchNumber); err != nil {
		return err
	}
	return nil
}

// GetProcessingContext returns the processing context for the given batch.
func (p *PostgresStorage) GetProcessingContext(ctx context.Context, batchNumber uint64, dbTx pgx.Tx) (*state.ProcessingContext, error) {
	const getProcessingContextSQL = "SELECT batch_num, global_exit_root, timestamp, coinbase, forced_batch_num from state.batch WHERE batch_num = $1"

	e := p.getExecQuerier(dbTx)
	row := e.QueryRow(ctx, getProcessingContextSQL, batchNumber)
	processingContext := state.ProcessingContext{}
	var (
		gerStr      string
		coinbaseStr string
	)
	if err := row.Scan(
		&processingContext.BatchNumber,
		&gerStr,
		&processingContext.Timestamp,
		&coinbaseStr,
		&processingContext.ForcedBatchNum,
	); errors.Is(err, pgx.ErrNoRows) {
		return nil, state.ErrStateNotSynchronized
	} else if err != nil {
		return nil, err
	}
	processingContext.GlobalExitRoot = common.HexToHash(gerStr)
	processingContext.Coinbase = common.HexToAddress(coinbaseStr)

	return &processingContext, nil
}

// GetStateRootByBatchNumber get state root by batch number
func (p *PostgresStorage) GetStateRootByBatchNumber(ctx context.Context, batchNum uint64, dbTx pgx.Tx) (common.Hash, error) {
	const query = "SELECT state_root FROM state.batch WHERE batch_num = $1"
	var stateRootStr string
	e := p.getExecQuerier(dbTx)
	err := e.QueryRow(ctx, query, batchNum).Scan(&stateRootStr)
	if errors.Is(err, pgx.ErrNoRows) {
		return common.Hash{}, state.ErrNotFound
	} else if err != nil {
		return common.Hash{}, err
	}
	return common.HexToHash(stateRootStr), nil
}

func (p *PostgresStorage) addressesToHex(addresses []common.Address) []string {
	converted := make([]string, 0, len(addresses))

	for _, address := range addresses {
		converted = append(converted, address.String())
	}

	return converted
}

func (p *PostgresStorage) hashesToHex(hashes []common.Hash) []string {
	converted := make([]string, 0, len(hashes))

	for _, hash := range hashes {
		converted = append(converted, hash.String())
	}

	return converted
}

// AddTrustedReorg is used to store trusted reorgs
func (p *PostgresStorage) AddTrustedReorg(ctx context.Context, reorg *state.TrustedReorg, dbTx pgx.Tx) error {
	const insertTrustedReorgSQL = "INSERT INTO state.trusted_reorg (timestamp, batch_num, reason) VALUES (NOW(), $1, $2)"

	e := p.getExecQuerier(dbTx)
	_, err := e.Exec(ctx, insertTrustedReorgSQL, reorg.BatchNumber, reorg.Reason)
	return err
}

// CountReorgs returns the number of reorgs
func (p *PostgresStorage) CountReorgs(ctx context.Context, dbTx pgx.Tx) (uint64, error) {
	const countReorgsSQL = "SELECT COUNT(*) FROM state.trusted_reorg"

	var count uint64
	q := p.getExecQuerier(dbTx)
	err := q.QueryRow(ctx, countReorgsSQL).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetReorgedTransactions returns the transactions that were reorged
func (p *PostgresStorage) GetReorgedTransactions(ctx context.Context, batchNumber uint64, dbTx pgx.Tx) ([]*types.Transaction, error) {
	const getReorgedTransactionsSql = "SELECT encoded FROM state.transaction t INNER JOIN state.l2block b ON t.l2_block_num = b.block_num WHERE b.batch_num >= $1 ORDER BY l2_block_num ASC"
	e := p.getExecQuerier(dbTx)
	rows, err := e.Query(ctx, getReorgedTransactionsSql, batchNumber)
	if !errors.Is(err, pgx.ErrNoRows) && err != nil {
		return nil, err
	}
	defer rows.Close()

	txs := make([]*types.Transaction, 0, len(rows.RawValues()))

	for rows.Next() {
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		var encodedTx string
		err := rows.Scan(&encodedTx)
		if err != nil {
			return nil, err
		}

		tx, err := state.DecodeTx(encodedTx)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, nil
}
