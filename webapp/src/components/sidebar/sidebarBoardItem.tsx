// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.
import React, {useState} from 'react'
import {useIntl} from 'react-intl'

import {Board} from '../../blocks/board'
import {BoardView, IViewType} from '../../blocks/boardView'
import mutator from '../../mutator'
import IconButton from '../../widgets/buttons/iconButton'
import DeleteIcon from '../../widgets/icons/delete'
import OptionsIcon from '../../widgets/icons/options'
import Menu from '../../widgets/menu'
import MenuWrapper from '../../widgets/menuWrapper'
import BoardPermissionGate from '../permissions/boardPermissionGate'

import './sidebarBoardItem.scss'
import {CategoryBlocks} from '../../store/sidebar'
import CreateNewFolder from '../../widgets/icons/newFolder'
import {useAppSelector} from '../../store/hooks'
import {getCurrentBoardViews, getCurrentViewId} from '../../store/views'
import Folder from '../../widgets/icons/folder'
import Check from '../../widgets/icons/checkIcon'
import BoardIcon from '../../widgets/icons/board'
import TableIcon from '../../widgets/icons/table'
import GalleryIcon from '../../widgets/icons/gallery'
import CalendarIcon from '../../widgets/icons/calendar'

import {getCurrentTeam} from '../../store/teams'
import {Permission} from '../../constants'

const iconForViewType = (viewType: IViewType): JSX.Element => {
    switch (viewType) {
    case 'board': return <BoardIcon/>
    case 'table': return <TableIcon/>
    case 'gallery': return <GalleryIcon/>
    case 'calendar': return <CalendarIcon/>
    default: return <div/>
    }
}

type Props = {
    isActive: boolean
    categoryBlocks: CategoryBlocks
    board: Board
    allCategories: Array<CategoryBlocks>
    onDeleteRequest: (board: Board) => void
    showBoard: (boardId: string) => void
    showView: (viewId: string, boardId: string) => void
}

const SidebarBoardItem = (props: Props) => {
    const intl = useIntl()

    const [boardsMenuOpen, setBoardsMenuOpen] = useState<{[key: string]: boolean}>({})

    const team = useAppSelector(getCurrentTeam)
    const boardViews = useAppSelector(getCurrentBoardViews)
    const currentViewId = useAppSelector(getCurrentViewId)
    const teamID = team?.id || ''

    const generateMoveToCategoryOptions = (blockID: string) => {
        return props.allCategories.map((category) => (
            <Menu.Text
                key={category.id}
                id={category.id}
                name={category.name}
                icon={category.id === props.categoryBlocks.id ? <Check/> : <Folder/>}
                onClick={async (toCategoryID) => {
                    const fromCategoryID = props.categoryBlocks.id
                    await mutator.moveBlockToCategory(teamID, blockID, toCategoryID, fromCategoryID)
                }}
            />
        ))
    }

    const board = props.board
    const title = board.title || intl.formatMessage({id: 'Sidebar.untitled-board', defaultMessage: '(Untitled Board)'})
    return (
        <>
            <div
                className={`SidebarBoardItem subitem ${props.isActive ? 'active' : ''}`}
                onClick={() => props.showBoard(board.id)}
            >
                <div className='octo-sidebar-icon'>
                    {board.icon}
                </div>
                <div
                    className='octo-sidebar-title'
                    title={title}
                >
                    {title}
                </div>
                <MenuWrapper
                    className={boardsMenuOpen[board.id] ? 'menuOpen' : 'x'}
                    stopPropagationOnToggle={true}
                    onToggle={(open) => {
                        setBoardsMenuOpen((menuState) => {
                            const newState = {...menuState}
                            newState[board.id] = open
                            return newState
                        })
                    }}
                >
                    <IconButton icon={<OptionsIcon/>}/>
                    <Menu
                        fixed={true}
                        position='left'
                    >
                        <BoardPermissionGate
                            boardId={board.id}
                            permissions={[Permission.DeleteBoard]}
                        >
                            <Menu.Text
                                key={`deleteBlock-${board.id}`}
                                id='deleteBlock'
                                name={intl.formatMessage({id: 'Sidebar.delete-board', defaultMessage: 'Delete Board'})}
                                icon={<DeleteIcon/>}
                                onClick={() => {
                                    props.onDeleteRequest(board)
                                }}
                            />
                        </BoardPermissionGate>
                        <Menu.SubMenu
                            key={`moveBlock-${board.id}`}
                            id='moveBlock'
                            name={intl.formatMessage({id: 'SidebarCategories.BlocksMenu.Move', defaultMessage: 'Move To...'})}
                            icon={<CreateNewFolder/>}
                            position='bottom'
                        >
                            {generateMoveToCategoryOptions(board.id)}
                        </Menu.SubMenu>
                    </Menu>
                </MenuWrapper>
            </div>
            {props.isActive && boardViews.map((view: BoardView) => (
                <div
                    key={view.id}
                    className={`SidebarBoardItem sidebar-view-item ${view.id === currentViewId ? 'active' : ''}`}
                    onClick={() => props.showView(view.id, board.id)}
                >
                    {iconForViewType(view.fields.viewType)}
                    <div
                        className='octo-sidebar-title'
                        title={view.title || intl.formatMessage({id: 'Sidebar.untitled-view', defaultMessage: '(Untitled View)'})}
                    >
                        {view.title || intl.formatMessage({id: 'Sidebar.untitled-view', defaultMessage: '(Untitled View)'})}
                    </div>
                </div>
            ))}
        </>
    )
}

export default React.memo(SidebarBoardItem)
