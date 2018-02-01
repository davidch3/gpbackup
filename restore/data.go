package restore

/*
 * This file contains structs and functions related to backing up data on the segments.
 */

import (
	"fmt"

	"github.com/greenplum-db/gp-common-go-libs/gplog"
	"github.com/greenplum-db/gpbackup/utils"
	"github.com/pkg/errors"
)

var (
	tableDelim = ","
)

func CopyTableIn(connection *utils.DBConn, tableName string, tableAttributes string, backupFile string, singleDataFile bool, whichConn int, oid uint32) int64 {
	whichConn = connection.ValidateConnNum(whichConn)
	usingCompression, compressionProgram := utils.GetCompressionParameters()
	copyCommand := ""
	if singleDataFile {
		copyCommand = fmt.Sprintf("PROGRAM 'cat %s'", fmt.Sprintf("%s_%d", backupFile, oid))
	} else if usingCompression && !singleDataFile {
		copyCommand = fmt.Sprintf("PROGRAM '%s < %s'", compressionProgram.DecompressCommand, backupFile)
	} else {
		copyCommand = fmt.Sprintf("'%s'", backupFile)
	}
	query := fmt.Sprintf("COPY %s%s FROM %s WITH CSV DELIMITER '%s' ON SEGMENT;", tableName, tableAttributes, copyCommand, tableDelim)
	result, err := connection.Exec(query, whichConn)
	if err != nil {
		logger.Fatal(err, "Error loading data into table %s", tableName)
	}
	numRows, _ := result.RowsAffected()
	return numRows
}

func restoreSingleTableData(entry utils.MasterDataEntry, tableNum uint32, totalTables int, whichConn int) {
	name := utils.MakeFQN(entry.Schema, entry.Name)
	if logger.GetVerbosity() > gplog.LOGINFO {
		// No progress bar at this log level, so we note table count here
		logger.Verbose("Reading data for table %s from file (table %d of %d)", name, tableNum, totalTables)
	} else {
		logger.Verbose("Reading data for table %s from file", name)
	}
	backupFile := ""
	if backupConfig.SingleDataFile {
		backupFile = fmt.Sprintf("%s_%d", globalCluster.GetSegmentPipePathForCopyCommand(), globalCluster.PID)
	} else {
		backupFile = globalCluster.GetTableBackupFilePathForCopyCommand(entry.Oid, backupConfig.SingleDataFile)
	}
	numRowsRestored := CopyTableIn(connection, name, entry.AttributeString, backupFile, backupConfig.SingleDataFile, whichConn, entry.Oid)
	numRowsBackedUp := entry.RowsCopied
	CheckRowsRestored(numRowsRestored, numRowsBackedUp, name)
}

func CheckRowsRestored(rowsRestored int64, rowsBackedUp int64, tableName string) {
	if rowsRestored != rowsBackedUp {
		rowsErrMsg := fmt.Sprintf("Expected to restore %d rows to table %s, but restored %d instead", rowsBackedUp, tableName, rowsRestored)
		if *onErrorContinue {
			logger.Error(rowsErrMsg)
		} else {
			agentErr := CheckAgentErrorsOnSegments(globalCluster)
			if agentErr != nil {
				logger.Error(rowsErrMsg)
				logger.Fatal(agentErr, "")
			}
			logger.Fatal(errors.Errorf("%s", rowsErrMsg), "")
		}
	}
}
