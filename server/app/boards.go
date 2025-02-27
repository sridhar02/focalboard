package app

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/mattermost/focalboard/server/model"
	"github.com/mattermost/focalboard/server/utils"
)

var (
	ErrBoardMemberIsLastAdmin = errors.New("cannot leave a board with no admins")
	ErrNewBoardCannotHaveID   = errors.New("new board cannot have an ID")
	ErrInsufficientLicense    = errors.New("appropriate license required")
)

func (a *App) GetBoard(boardID string) (*model.Board, error) {
	board, err := a.store.GetBoard(boardID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return board, nil
}

func (a *App) GetBoardMetadata(boardID string) (*model.Board, *model.BoardMetadata, error) {
	license := a.store.GetLicense()
	if license == nil || !(*license.Features.Compliance) {
		return nil, nil, ErrInsufficientLicense
	}

	board, err := a.GetBoard(boardID)
	if err != nil {
		return nil, nil, err
	}
	if board == nil {
		// Board may have been deleted, retrieve most recent history instead
		board, err = a.getBoardHistory(boardID, true)
		if err != nil {
			return nil, nil, err
		}
	}

	if board == nil {
		// Board not found
		return nil, nil, nil
	}

	earliestTime, _, err := a.getBoardDescendantModifiedInfo(boardID, false)
	if err != nil {
		return nil, nil, err
	}

	latestTime, lastModifiedBy, err := a.getBoardDescendantModifiedInfo(boardID, true)
	if err != nil {
		return nil, nil, err
	}

	boardMetadata := model.BoardMetadata{
		BoardID:                 boardID,
		DescendantFirstUpdateAt: earliestTime,
		DescendantLastUpdateAt:  latestTime,
		CreatedBy:               board.CreatedBy,
		LastModifiedBy:          lastModifiedBy,
	}
	return board, &boardMetadata, nil
}

func (a *App) getBoardHistory(boardID string, latest bool) (*model.Board, error) {
	opts := model.QueryBlockHistoryOptions{
		Limit:      1,
		Descending: latest,
	}
	boards, err := a.store.GetBoardHistory(boardID, opts)
	if err != nil {
		return nil, fmt.Errorf("could not get history for board: %w", err)
	}
	if len(boards) == 0 {
		return nil, nil
	}

	return boards[0], nil
}

func (a *App) getBoardDescendantModifiedInfo(boardID string, latest bool) (int64, string, error) {
	board, err := a.getBoardHistory(boardID, latest)
	if err != nil {
		return 0, "", err
	}
	if board == nil {
		return 0, "", fmt.Errorf("history not found for board: %w", err)
	}

	var timestamp int64
	modifiedBy := board.ModifiedBy
	if latest {
		timestamp = board.UpdateAt
	} else {
		timestamp = board.CreateAt
	}

	// use block_history to fetch blocks in case they were deleted and no longer exist in blocks table.
	opts := model.QueryBlockHistoryOptions{
		Limit:      1,
		Descending: latest,
	}
	blocks, err := a.store.GetBlockHistoryDescendants(boardID, opts)
	if err != nil {
		return 0, "", fmt.Errorf("could not get blocks history descendants for board: %w", err)
	}
	if len(blocks) > 0 {
		// Compare the board history info with the descendant block info, if it exists
		block := &blocks[0]
		if latest && block.UpdateAt > timestamp {
			timestamp = block.UpdateAt
			modifiedBy = block.ModifiedBy
		} else if !latest && block.CreateAt < timestamp {
			timestamp = block.CreateAt
			modifiedBy = block.ModifiedBy
		}
	}
	return timestamp, modifiedBy, nil
}

func (a *App) DuplicateBoard(boardID, userID, toTeam string, asTemplate bool) (*model.BoardsAndBlocks, []*model.BoardMember, error) {
	bab, members, err := a.store.DuplicateBoard(boardID, userID, toTeam, asTemplate)
	if err != nil {
		return nil, nil, err
	}
	go func() {
		teamID := ""
		for _, board := range bab.Boards {
			teamID = board.TeamID
			a.wsAdapter.BroadcastBoardChange(teamID, board)
		}
		for _, block := range bab.Blocks {
			a.wsAdapter.BroadcastBlockChange(teamID, block)
		}
		for _, member := range members {
			a.wsAdapter.BroadcastMemberChange(teamID, member.BoardID, member)
		}
	}()
	return bab, members, err
}

func (a *App) GetBoardsForUserAndTeam(userID, teamID string) ([]*model.Board, error) {
	return a.store.GetBoardsForUserAndTeam(userID, teamID)
}

func (a *App) GetTemplateBoards(teamID string) ([]*model.Board, error) {
	return a.store.GetTemplateBoards(teamID)
}

func (a *App) CreateBoard(board *model.Board, userID string, addMember bool) (*model.Board, error) {
	if board.ID != "" {
		return nil, ErrNewBoardCannotHaveID
	}
	board.ID = utils.NewID(utils.IDTypeBoard)

	var newBoard *model.Board
	var member *model.BoardMember
	var err error
	if addMember {
		newBoard, member, err = a.store.InsertBoardWithAdmin(board, userID)
	} else {
		newBoard, err = a.store.InsertBoard(board, userID)
	}

	if err != nil {
		return nil, err
	}

	go func() {
		a.wsAdapter.BroadcastBoardChange(newBoard.TeamID, newBoard)

		if addMember {
			a.wsAdapter.BroadcastMemberChange(newBoard.TeamID, newBoard.ID, member)
		}
	}()

	return newBoard, nil
}

func (a *App) PatchBoard(patch *model.BoardPatch, boardID, userID string) (*model.Board, error) {
	updatedBoard, err := a.store.PatchBoard(boardID, patch, userID)
	if err != nil {
		return nil, err
	}

	go func() {
		a.wsAdapter.BroadcastBoardChange(updatedBoard.TeamID, updatedBoard)
	}()

	return updatedBoard, nil
}

func (a *App) DeleteBoard(boardID, userID string) error {
	board, err := a.store.GetBoard(boardID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := a.store.DeleteBoard(boardID, userID); err != nil {
		return err
	}

	go func() {
		a.wsAdapter.BroadcastBoardDelete(board.TeamID, boardID)
	}()

	return nil
}

func (a *App) GetMembersForBoard(boardID string) ([]*model.BoardMember, error) {
	return a.store.GetMembersForBoard(boardID)
}

func (a *App) GetMembersForUser(userID string) ([]*model.BoardMember, error) {
	return a.store.GetMembersForUser(userID)
}

func (a *App) AddMemberToBoard(member *model.BoardMember) (*model.BoardMember, error) {
	board, err := a.store.GetBoard(member.BoardID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	existingMembership, err := a.store.GetMemberForBoard(member.BoardID, member.UserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if existingMembership != nil {
		return existingMembership, nil
	}

	newMember, err := a.store.SaveMember(member)
	if err != nil {
		return nil, err
	}

	go func() {
		a.wsAdapter.BroadcastMemberChange(board.TeamID, member.BoardID, member)
	}()

	return newMember, nil
}

func (a *App) UpdateBoardMember(member *model.BoardMember) (*model.BoardMember, error) {
	board, bErr := a.store.GetBoard(member.BoardID)
	if errors.Is(bErr, sql.ErrNoRows) {
		return nil, nil
	}
	if bErr != nil {
		return nil, bErr
	}

	oldMember, err := a.store.GetMemberForBoard(member.BoardID, member.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// if we're updating an admin, we need to check that there is at
	// least still another admin on the board
	if oldMember.SchemeAdmin && !member.SchemeAdmin {
		isLastAdmin, err2 := a.isLastAdmin(member.UserID, member.BoardID)
		if err2 != nil {
			return nil, err2
		}
		if isLastAdmin {
			return nil, ErrBoardMemberIsLastAdmin
		}
	}

	newMember, err := a.store.SaveMember(member)
	if err != nil {
		return nil, err
	}

	go func() {
		a.wsAdapter.BroadcastMemberChange(board.TeamID, member.BoardID, member)
	}()

	return newMember, nil
}

func (a *App) isLastAdmin(userID, boardID string) (bool, error) {
	members, err := a.store.GetMembersForBoard(boardID)
	if err != nil {
		return false, err
	}

	for _, m := range members {
		if m.SchemeAdmin && m.UserID != userID {
			return false, nil
		}
	}
	return true, nil
}

func (a *App) DeleteBoardMember(boardID, userID string) error {
	board, bErr := a.store.GetBoard(boardID)
	if errors.Is(bErr, sql.ErrNoRows) {
		return nil
	}
	if bErr != nil {
		return bErr
	}

	oldMember, err := a.store.GetMemberForBoard(boardID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	// if we're removing an admin, we need to check that there is at
	// least still another admin on the board
	if oldMember.SchemeAdmin {
		isLastAdmin, err := a.isLastAdmin(userID, boardID)
		if err != nil {
			return err
		}
		if isLastAdmin {
			return ErrBoardMemberIsLastAdmin
		}
	}

	if err := a.store.DeleteMember(boardID, userID); err != nil {
		return err
	}

	go func() {
		a.wsAdapter.BroadcastMemberDelete(board.TeamID, boardID, userID)
	}()

	return nil
}

func (a *App) SearchBoardsForUserAndTeam(term, userID, teamID string) ([]*model.Board, error) {
	return a.store.SearchBoardsForUserAndTeam(term, userID, teamID)
}
