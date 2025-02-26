import axios from 'axios';
import { AlertCircle, ArrowLeft, ArrowRight, Braces, Clock, Copy, History, Loader, Pencil, Play, Send, Table, X, XCircle } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { useStream } from '../../contexts/StreamContext';
import chatService from '../../services/chatService';
import ConfirmationModal from '../modals/ConfirmationModal';
import RollbackConfirmationModal from '../modals/RollbackConfirmationModal';
import LoadingSteps from './LoadingSteps';
import { Message, QueryResult } from './types';

interface QueryState {
    isExecuting: boolean;
    isExample: boolean;
}

interface QueryResultState {
    data: any;
    loading: boolean;
    error: string | null;
    currentPage: number;
    pageSize: number;
    totalRecords: number | null;
}

interface PageData {
    data: any[];
    totalRecords: number;
}

interface MessageTileProps {
    chatId: string;
    message: Message;
    setMessage: (message: Message) => void;
    checkSSEConnection: () => Promise<void>;
    onEdit?: (id: string) => void;
    editingMessageId: string | null;
    editInput: string;
    setEditInput: (input: string) => void;
    onSaveEdit: (id: string, content: string) => void;
    onCancelEdit: () => void;
    queryStates: Record<string, QueryState>;
    setQueryStates: React.Dispatch<React.SetStateAction<Record<string, QueryState>>>;
    queryTimeouts: React.MutableRefObject<Record<string, NodeJS.Timeout>>;
    isFirstMessage?: boolean;
    onQueryUpdate: (callback: () => void) => void;
    onEditQuery: (id: string, queryId: string, query: string) => void;
}

const toastStyle = {
    style: {
        background: '#000',
        color: '#fff',
        border: '4px solid #000',
        borderRadius: '12px',
        boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
        padding: '12px 24px',
        fontSize: '14px',
        fontWeight: '500',
    },
    position: 'bottom-center' as const,
    duration: 2000,
};

const formatMessageTime = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleTimeString('en-US', {
        hour: 'numeric',
        minute: 'numeric',
        hour12: true
    });
};

const DEFAULT_PAGE_SIZE = 25; // Set page size to 25 items per page

export default function MessageTile({
    chatId,
    message,
    setMessage,
    onEdit,
    editingMessageId,
    editInput,
    setEditInput,
    onSaveEdit,
    onCancelEdit,
    queryStates,
    setQueryStates,
    queryTimeouts,
    checkSSEConnection,
    isFirstMessage,
    onQueryUpdate,
    onEditQuery
}: MessageTileProps) {
    const { streamId } = useStream();
    const [viewMode, setViewMode] = useState<'table' | 'json'>('table');
    const [showCriticalConfirm, setShowCriticalConfirm] = useState(false);
    const [queryToExecute, setQueryToExecute] = useState<string | null>(null);
    const [rollbackState, setRollbackState] = useState<{
        show: boolean;
        queryId: string | null;
    }>({ show: false, queryId: null });
    const [streamingQueryIndex, setStreamingQueryIndex] = useState<number>(-1);
    const [isDescriptionStreaming, setIsDescriptionStreaming] = useState(false);
    const [isQueryStreaming, setIsQueryStreaming] = useState(false);
    const [currentDescription, setCurrentDescription] = useState('');
    const [currentQuery, setCurrentQuery] = useState('');
    const abortControllerRef = useRef<Record<string, AbortController>>({});
    const [queryResults, setQueryResults] = useState<Record<string, QueryResultState>>({});
    const pageDataCacheRef = useRef<Record<string, Record<number, PageData>>>({});

    useEffect(() => {
        const streamQueries = async () => {
            if (!message.queries || !message.is_streaming) return;

            // Just set the content immediately without streaming
            for (let i = 0; i < message.queries.length; i++) {
                const query = message.queries[i];
                setStreamingQueryIndex(i);
                setCurrentDescription(query.description);
                setCurrentQuery(query.query);

                // Keep the existing query state management
                if (message.queries) {
                    message.queries[i].is_streaming = false;
                }
            }
            setStreamingQueryIndex(-1);
        };

        streamQueries();
    }, [message.queries, message.is_streaming]);

    useEffect(() => {
        if (message.queries) {
            const initialStates: Record<string, QueryResultState> = {};
            message.queries.forEach(query => {
                if (!queryResults[query.id]) {
                    const resultArray = parseResults(query.execution_result || []);
                    const totalRecords = query.pagination?.total_records_count || resultArray.length;

                    // For initial data, always show first page (25 records)
                    const pageData = resultArray.slice(0, 25);

                    // Cache both pages from initial 50 records if available
                    if (!pageDataCacheRef.current[query.id]) {
                        pageDataCacheRef.current[query.id] = {};
                    }

                    if (resultArray.length > 0) {
                        // Cache first page (1-25)
                        pageDataCacheRef.current[query.id][1] = {
                            data: resultArray.slice(0, 25),
                            totalRecords
                        };

                        // Cache second page (26-50) if it exists
                        if (resultArray.length > 25) {
                            pageDataCacheRef.current[query.id][2] = {
                                data: resultArray.slice(25, 50),
                                totalRecords
                            };
                        }
                    }

                    initialStates[query.id] = {
                        data: pageData,
                        loading: false,
                        error: null,
                        currentPage: 1,
                        pageSize: 25,
                        totalRecords: totalRecords
                    };
                }
            });
            if (Object.keys(initialStates).length > 0) {
                setQueryResults(prev => ({ ...prev, ...initialStates }));
            }
        }
    }, [message.queries]);

    const handleCopyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text);
        toast('Copied to clipboard!', {
            ...toastStyle,
            icon: '📋',
        });
    };

    const handleExecuteQuery = async (queryId: string) => {
        const query = message.queries?.find(q => q.id === queryId);
        if (!query) return;

        if (query.is_critical) {
            setQueryToExecute(queryId);
            setShowCriticalConfirm(true);
            return;
        }
        executeQuery(queryId);
    };

    const executeQuery = async (queryId: string) => {
        const query = message.queries?.find(q => q.id === queryId);
        if (!query) return;

        // Clear any existing timeout
        if (queryTimeouts.current[queryId]) {
            clearTimeout(queryTimeouts.current[queryId]);
            delete queryTimeouts.current[queryId];
        }

        // Create new AbortController for this query
        abortControllerRef.current[queryId] = new AbortController();

        onQueryUpdate(() => {
            setQueryStates(prev => ({
                ...prev,
                [queryId]: { isExecuting: true, isExample: false }
            }));
        });

        try {
            await checkSSEConnection();
            const response = await chatService.executeQuery(
                chatId,
                message.id,
                queryId,
                streamId || '',
                abortControllerRef.current[queryId]
            );

            console.log('executeQuery response', response?.success);
            if (response?.success) {

                const fullData = parseResults(response.data.execution_result);
                const totalRecords = response.data.total_records_count;

                // Slice the data into pages of 25
                const pageData = sliceIntoPages(fullData, DEFAULT_PAGE_SIZE, 1);

                // Cache both pages from the API response
                const basePage = Math.floor((1 - 1) / 2) * 2 + 1;
                const firstHalf = sliceIntoPages(fullData, DEFAULT_PAGE_SIZE, 1);
                const secondHalf = sliceIntoPages(fullData, DEFAULT_PAGE_SIZE, 2);

                pageDataCacheRef.current[queryId][basePage] = {
                    data: firstHalf,
                    totalRecords
                };
                pageDataCacheRef.current[queryId][basePage + 1] = {
                    data: secondHalf,
                    totalRecords
                };
                onQueryUpdate(() => {

                    setMessage({
                        ...message,
                        queries: message.queries?.map(q => q.id === queryId ? {
                            ...q,
                            is_executed: response.data.is_executed,
                            is_rolled_back: response.data.is_rolled_back,
                            execution_result: response.data.execution_result,
                            execution_time: response.data.execution_time,
                            error: response.data.error,
                            total_records_count: response.data.total_records_count,
                            pagination: {
                                ...q.pagination,
                                total_records_count: response.data.total_records_count,
                            }
                        } : q)
                    });

                    setQueryResults(prev => ({
                        ...prev,
                        [queryId]: {
                            ...prev[queryId],
                            data: pageData,
                            loading: false,
                            error: null,
                            currentPage: 1,
                            pageSize: DEFAULT_PAGE_SIZE,
                            totalRecords: totalRecords
                        }
                    }));
                });

                toast('Query executed!', {
                    ...toastStyle,
                    icon: '✅',
                });
            }
        } catch (error: any) {
            // Only show error if not aborted
            if (error.name !== 'AbortError') {
                console.log('error', error.message);
                toast.error("Query execution failed: " + error);
            }
        } finally {
            onQueryUpdate(() => {
                setQueryStates(prev => ({
                    ...prev,
                    [queryId]: { isExecuting: false, isExample: !query.is_executed }
                }));
            });
            // Clean up abort controller
            delete abortControllerRef.current[queryId];
        }
    };

    const handleRollback = async (queryId: string) => {
        const queryIndex = message.queries?.findIndex(q => q.id === queryId) ?? -1;
        if (queryIndex === -1) return;

        try {
            // Create new AbortController for this query
            abortControllerRef.current[queryId] = new AbortController();
            onQueryUpdate(() => {
                setQueryStates(prev => ({
                    ...prev,
                    [queryId]: { isExecuting: true, isExample: true }
                }));
            });

            await checkSSEConnection();
            const response = await chatService.rollbackQuery(chatId, message.id, queryId, streamId || '', abortControllerRef.current[queryId]);
            console.log('rolledBack', response);

            if (response?.success) {
                setMessage({
                    ...message,
                    queries: message.queries?.map(q => q.id === queryId ? {
                        ...q,
                        is_executed: true,
                        is_rolled_back: true,
                        execution_result: response?.data?.execution_result,
                        execution_time: response?.data?.execution_time,
                        error: response?.data?.error,
                    } : q)
                });
                toast('Changes reverted', {
                    ...toastStyle,
                    icon: '↺',
                });
                onQueryUpdate(() => {
                    if (message.queries) {
                        message.queries[queryIndex].is_rolled_back = true;
                    }
                });
            }
        } catch (error: any) {
            toast.error(error.message);
        } finally {
            onQueryUpdate(() => {
                setQueryStates(prev => ({
                    ...prev,
                    [queryId]: { isExecuting: false, isExample: true, isRolledBack: false }
                }));
            });
            setRollbackState({ show: false, queryId: null });
            delete abortControllerRef.current[queryId];
        }
    };

    const renderTableView = (data: any[]) => {
        if (!data || data.length === 0) {
            return <div className="text-gray-500">No data to display</div>;
        }

        const columns = Object.keys(data[0]);

        return (
            <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse">
                    <thead>
                        <tr>
                            {columns.map(column => (
                                <th key={column} className="py-2 px-4 bg-gray-800 border-b border-gray-700 text-gray-300 font-mono">
                                    {column}
                                </th>
                            ))}
                        </tr>
                    </thead>
                    <tbody>
                        {data.map((row, i) => (
                            <tr key={i} className="border-b border-gray-700">
                                {columns.map(column => (
                                    <td key={column} className="py-2 px-4">
                                        <span className={`${typeof row[column] === 'number'
                                            ? 'text-cyan-400'
                                            : typeof row[column] === 'boolean'
                                                ? 'text-purple-400'
                                                : column.includes('time') || column.includes('date')
                                                    ? 'text-yellow-300'
                                                    : 'text-green-400'
                                            }`}>
                                            {JSON.stringify(row[column])}
                                        </span>
                                    </td>
                                ))}
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        );
    };

    const renderQueryResult = (result: any) => {
        if (!result) {
            return <div className="text-gray-500">No results available</div>;
        }

        const query = message.queries?.find(q => q.execution_result === result);
        if (!query) return null;

        const state = queryResults[query.id];
        if (!state) return null;

        // Parse the data - handle both array and object with results property
        const parsedData = parseResults(state.data);
        const totalRecords = state.totalRecords || parsedData.length;
        const showPagination = totalRecords > state.pageSize;

        // Don't slice the data here since we're getting paginated data from API
        const currentPageData = parsedData;

        return (
            <div className="query-results">
                {totalRecords > 50 && (
                    <div className="text-gray-300 mb-4">
                        The result contains total <b className="text-yellow-500">{totalRecords}</b> records.
                    </div>
                )}

                {state.loading ? (
                    <div className="flex justify-center p-4">
                        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-white"></div>
                    </div>
                ) : (
                    <>
                        {state.error && (
                            <div className="text-red-500 py-2 mb-2">
                                Error in fetching results: {state.error}
                            </div>
                        )}

                        {viewMode === 'table' ? (
                            currentPageData.length > 0 ? (
                                renderTableView(currentPageData)
                            ) : (
                                <div className="text-gray-500">No data to display</div>
                            )
                        ) : (
                            <pre className="overflow-x-auto whitespace-pre-wrap">
                                {JSON.stringify(currentPageData, null, 2)}
                            </pre>
                        )}

                        {showPagination && (
                            <div className="flex justify-center mt-6">
                                <div className="flex items-center gap-4 bg-gray-800 rounded-lg p-1.5">
                                    <button
                                        onClick={() => handlePageChange(query.id, state.currentPage - 1)}
                                        disabled={state.currentPage === 1}
                                        className="
                                            flex items-center justify-center
                                            w-8 h-8
                                            rounded
                                            transition-colors
                                            disabled:opacity-40
                                            disabled:cursor-not-allowed
                                            enabled:hover:bg-gray-700
                                            enabled:active:bg-gray-600
                                        "
                                        title="Previous page"
                                    >
                                        <ArrowLeft className="w-4 h-4" />
                                    </button>

                                    <div className="flex items-center gap-2 text-sm font-mono">
                                        <span className="text-gray-400">Page</span>
                                        <span className="bg-gray-700 rounded px-2 py-1 min-w-[2rem] text-center">
                                            {state.currentPage}
                                        </span>
                                        <span className="text-gray-400">of</span>
                                        <span className="bg-gray-700 rounded px-2 py-1 min-w-[2rem] text-center">
                                            {Math.max(1, Math.ceil(totalRecords / state.pageSize))}
                                        </span>
                                    </div>

                                    <button
                                        onClick={() => handlePageChange(query.id, state.currentPage + 1)}
                                        disabled={state.currentPage >= Math.ceil(totalRecords / state.pageSize)}
                                        className="
                                            flex items-center justify-center
                                            w-8 h-8
                                            rounded
                                            transition-colors
                                            disabled:opacity-40
                                            disabled:cursor-not-allowed
                                            enabled:hover:bg-gray-700
                                            enabled:active:bg-gray-600
                                        "
                                        title="Next page"
                                    >
                                        <ArrowRight className="w-4 h-4" />
                                    </button>
                                </div>
                            </div>
                        )}
                    </>
                )}
            </div>
        );
    };

    const sliceIntoPages = (data: any[], pageSize: number, currentPage: number): any[] => {
        const startIndex = (currentPage % 2 === 1) ? 0 : pageSize;
        return data.slice(startIndex, startIndex + pageSize);
    };

    const preserveScroll = (callback: () => void) => {
        // Find the closest scrollable container
        const scrollContainer = document.querySelector('[data-chat-container]');
        if (!scrollContainer) return callback();

        const oldHeight = scrollContainer.scrollHeight;
        const oldScroll = scrollContainer.scrollTop;
        const wasAtBottom = scrollContainer.scrollHeight - scrollContainer.scrollTop - scrollContainer.clientHeight < 10;

        callback();

        requestAnimationFrame(() => {
            if (wasAtBottom) {
                scrollContainer.scrollTop = scrollContainer.scrollHeight;
            } else {
                const newHeight = scrollContainer.scrollHeight;
                const heightDiff = newHeight - oldHeight;
                scrollContainer.scrollTop = oldScroll + heightDiff;
            }
        });
    };

    const handlePageChange = useCallback(async (queryId: string, page: number) => {
        const query = message.queries?.find(q => q.id === queryId);
        if (!query) return;

        const state = queryResults[queryId];
        const newOffset = (page - 1) * state.pageSize;

        // Initialize page data cache for this query if it doesn't exist
        if (!pageDataCacheRef.current[queryId]) {
            pageDataCacheRef.current[queryId] = {};
        }

        setQueryResults(prev => ({
            ...prev,
            [queryId]: { ...prev[queryId], loading: true, error: null }
        }));

        try {
            // Wrap state updates in preserveScroll
            if (newOffset < 50 && query.execution_result) {
                const resultArray = parseResults(query.execution_result);
                const totalRecords = query.pagination?.total_records_count || resultArray.length;

                // Calculate the slice for current page
                const startIndex = (page - 1) * state.pageSize;
                const endIndex = Math.min(startIndex + state.pageSize, resultArray.length);
                const pageData = resultArray.slice(startIndex, endIndex);

                // Cache the page data
                pageDataCacheRef.current[queryId][page] = {
                    data: pageData,
                    totalRecords
                };

                onQueryUpdate(() => {
                    setQueryResults(prev => ({
                        ...prev,
                        [queryId]: {
                            ...prev[queryId],
                            loading: false,
                            currentPage: page,
                            data: pageData,
                            error: null,
                            totalRecords
                        }
                    }));
                });
                return;
            }

            if (pageDataCacheRef.current[queryId][page]) {
                const cachedData = pageDataCacheRef.current[queryId][page];
                onQueryUpdate(() => {
                    setQueryResults(prev => ({
                        ...prev,
                        [queryId]: {
                            ...prev[queryId],
                            loading: false,
                            currentPage: page,
                            data: cachedData.data,
                            error: null,
                            totalRecords: cachedData.totalRecords
                        }
                    }));
                });
                return;
            }

            // For remote pagination - fetch new pages
            const apiPage = Math.ceil(page / 2); // Convert UI page to API page (each API call gets 50 records)
            const response = await axios.post(`${import.meta.env.VITE_API_URL}/chats/${chatId}/queries/results`, {
                message_id: message.id,
                query_id: queryId,
                stream_id: streamId,
                offset: (apiPage - 1) * 50 // Adjust offset to fetch 50 records at a time
            });

            // Get the results array from the response
            const responseData = response.data.data;
            const fullData = parseResults(responseData.execution_result);
            const totalRecords = responseData.total_records_count;

            // Slice the data into pages of 25
            const pageData = sliceIntoPages(fullData, state.pageSize, page % 2);

            // Cache both pages from the API response
            const basePage = Math.floor((page - 1) / 2) * 2 + 1;
            const firstHalf = sliceIntoPages(fullData, state.pageSize, 1);
            const secondHalf = sliceIntoPages(fullData, state.pageSize, 2);

            pageDataCacheRef.current[queryId][basePage] = {
                data: firstHalf,
                totalRecords
            };
            pageDataCacheRef.current[queryId][basePage + 1] = {
                data: secondHalf,
                totalRecords
            };

            // Wrap the final state update
            onQueryUpdate(() => {
                setQueryResults(prev => ({
                    ...prev,
                    [queryId]: {
                        ...prev[queryId],
                        loading: false,
                        currentPage: page,
                        data: pageData,
                        error: null,
                        totalRecords
                    }
                }));
            });

        } catch (error: any) {
            console.error('Error fetching results:', error);
            setQueryResults(prev => ({
                ...prev,
                [queryId]: {
                    ...prev[queryId],
                    loading: false,
                    error: error.response?.data?.error || 'Failed to fetch results',
                    data: prev[queryId].data
                }
            }));
        }
    }, [message.queries, queryResults, streamId, chatId, message.id, onQueryUpdate]);

    // Update parseResults to better handle the API response format
    const parseResults = (result: any): any[] => {
        if (!result) return [];

        if (Array.isArray(result)) {
            return result;
        }

        if (result && typeof result === 'object') {
            if ('results' in result) {
                return result.results;
            }
            // If no results property found, try to convert the object itself to array
            return [result];
        }

        return [result];
    };

    const renderQuery = (isMessageStreaming: boolean, query: QueryResult, index: number) => {
        const queryId = query.id;
        const shouldShowExampleResult = !query.is_executed && !query.is_rolled_back;
        const resultToShow = shouldShowExampleResult ? query.example_result : query.execution_result;
        const isCurrentlyStreaming = !isMessageStreaming && streamingQueryIndex === index;
        const [isEditingQuery, setIsEditingQuery] = useState(false);
        const [editedQueryText, setEditedQueryText] = useState(
            removeDuplicateQueries(query.query)
        );

        const shouldShowRollback = query.can_rollback &&
            query.is_executed &&
            !query.is_rolled_back;

        return (
            <div>
                <p className="mb-4 mt-4 font-base text-base">
                    <span className="text-black font-semibold">Explanation:</span> {isCurrentlyStreaming && isDescriptionStreaming
                        ? currentDescription
                        : query.description}
                </p>
                <div key={index} className="mt-4 bg-black text-white rounded-lg font-mono text-sm overflow-hidden w-full" style={{ minWidth: '100%' }}>
                    <div className="flex flex-wrap items-center justify-between gap-2 mb-4 px-4 pt-4">
                        <div className="flex items-center gap-2">
                            <span>Query {index + 1}:</span>
                            {query.is_edited && (
                                <span className="text-xs bg-gray-500/20 text-gray-300 px-2 py-0.5 rounded">
                                    Edited
                                </span>
                            )}
                        </div>
                        <div className="flex items-center">
                            {(
                                !queryStates[queryId]?.isExecuting && !query.is_executed && (
                                    <>
                                        <button
                                            onClick={(e) => {
                                                e.preventDefault();
                                                e.stopPropagation();
                                                setIsEditingQuery(true);
                                            }}
                                            className="p-2 hover:bg-gray-800 rounded transition-colors text-yellow-400 hover:text-yellow-300"
                                            title="Edit query"
                                        >
                                            <Pencil className="w-4 h-4" />
                                        </button>
                                        <div className="w-px h-4 bg-gray-700 mx-2" />
                                    </>
                                )
                            )}

                            {queryStates[queryId]?.isExecuting ? (
                                <button
                                    onClick={(e) => {
                                        e.preventDefault();
                                        e.stopPropagation();

                                        // Abort the API call if it's in progress
                                        if (abortControllerRef.current[queryId]) {
                                            abortControllerRef.current[queryId].abort();
                                            delete abortControllerRef.current[queryId];
                                        }

                                        // Clear any timeouts
                                        if (queryTimeouts.current[queryId]) {
                                            clearTimeout(queryTimeouts.current[queryId]);
                                            delete queryTimeouts.current[queryId];
                                        }

                                        // Update state
                                        onQueryUpdate(() => {
                                            setQueryStates(prev => ({
                                                ...prev,
                                                [queryId]: { isExecuting: false, isExample: !query.is_executed }
                                            }));
                                        });

                                        setTimeout(() => {
                                            window.scrollTo(window.scrollX, window.scrollY);
                                        }, 0);
                                        toast.error('Query cancelled', toastStyle);
                                    }}
                                    className="p-2 hover:bg-gray-800 rounded transition-colors text-red-500 hover:text-red-400"
                                    title="Cancel query"
                                >
                                    <XCircle className="w-4 h-4" />
                                </button>
                            ) : (
                                <button
                                    onClick={() => handleExecuteQuery(queryId)}
                                    className="p-2 text-red-500 hover:text-red-400 hover:bg-gray-800 rounded transition-colors"
                                    title="Execute query"
                                >
                                    <Play className="w-4 h-4" />
                                </button>
                            )}
                            <div className="w-px h-4 bg-gray-700 mx-2" />
                            <button
                                onClick={() => handleCopyToClipboard(query.query)}
                                className="p-2 hover:bg-gray-800 rounded text-white hover:text-gray-200"
                                title="Copy query"
                            >
                                <Copy className="w-4 h-4" />
                            </button>
                        </div>
                    </div>
                    {isEditingQuery ? (
                        <div className="px-4 pb-4 border-t border-gray-700 pt-4">
                            <textarea
                                value={editedQueryText}
                                onChange={(e) => setEditedQueryText(e.target.value)}
                                className="w-full bg-gray-900 text-white p-3 rounded-none 
                                       border-4 border-gray-600 font-mono text-sm min-h-[120px]
                                       focus:outline-none focus:border-yellow-500 shadow-[4px_4px_0px_0px_rgba(75,85,99,1)]"
                            />
                            <div className="flex justify-end gap-3 mt-4">
                                <button
                                    onClick={() => {
                                        setIsEditingQuery(false);
                                        setEditedQueryText(query.query);
                                    }}
                                    className="px-4 py-2 bg-gray-800 text-white border-2 border-gray-600
                                                  hover:bg-gray-700 transition-colors shadow-[2px_2px_0px_0px_rgba(75,85,99,1)]
                                                  active:translate-y-[1px] active:shadow-[1px_1px_0px_0px_rgba(75,85,99,1)]"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={() => {
                                        setIsEditingQuery(false);
                                        onEditQuery(message.id, queryId, editedQueryText);
                                    }}
                                    className="px-4 py-2 bg-yellow-400 text-black border-2 border-black
                                                  hover:bg-yellow-300 transition-colors shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]
                                                  active:translate-y-[1px] active:shadow-[1px_1px_0px_0px_rgba(0,0,0,1)]"
                                >
                                    Save Changes
                                </button>
                            </div>
                        </div>
                    ) : (
                        <pre className={`
                    text-sm overflow-x-auto p-4 border-t border-gray-700
                        ${isCurrentlyStreaming && isQueryStreaming ? 'animate-pulse duration-300' : ''}
                    `}>
                            <code className="whitespace-pre-wrap break-words">
                                {isCurrentlyStreaming && isQueryStreaming
                                    ? removeDuplicateQueries(currentQuery)
                                    : removeDuplicateQueries(query.query)}
                            </code>
                        </pre>
                    )}
                    {(query.execution_result || query.example_result || query.error) && (
                        <div className="border-t border-gray-700 mt-2 w-full">
                            {queryStates[queryId]?.isExecuting ? (
                                <div className="flex items-center justify-center p-8">
                                    <Loader className="w-8 h-8 animate-spin text-gray-400" />
                                    <span className="ml-3 text-gray-400">Executing  query...</span>
                                </div>
                            ) : (
                                <div className="mt-3 px-4 pt-4 w-full">
                                    <div className="flex flex-wrap items-center justify-between gap-2 mb-4">
                                        <div className="flex items-center gap-2 text-gray-400">
                                            {query.error ? (
                                                <span className="text-neo-error font-medium flex items-center gap-2">
                                                    <AlertCircle className="w-4 h-4" />
                                                    Error
                                                </span>
                                            ) : (
                                                <span>
                                                    {shouldShowExampleResult ? 'Example Result:' : query.is_rolled_back ? 'Rolled Back Result:' : 'Result:'}
                                                </span>
                                            )}
                                            {query.example_execution_time && !query.execution_time && !query.is_executed && !query.error && (
                                                <span className="text-xs bg-gray-800 px-2 py-1 rounded flex items-center gap-1">
                                                    <Clock className="w-3 h-3" />
                                                    {query.example_execution_time.toLocaleString()}ms
                                                </span>
                                            )}

                                            {query.execution_time! > 0 && !query.error && (
                                                <span className="text-xs bg-gray-800 px-2 py-1 rounded flex items-center gap-1">
                                                    <Clock className="w-3 h-3" />
                                                    {query.execution_time!.toLocaleString()}ms
                                                </span>
                                            )}
                                        </div>
                                        {!query.error && <div className="flex gap-2">
                                            <div className="flex items-center">
                                                <button
                                                    onClick={(e) => {
                                                        e.preventDefault();
                                                        e.stopPropagation();
                                                        setViewMode('table');
                                                        setTimeout(() => {
                                                            window.scrollTo(window.scrollX, window.scrollY);
                                                        }, 0);
                                                    }}
                                                    className={`p-1 md:p-2 rounded ${viewMode === 'table' ? 'bg-gray-700' : 'hover:bg-gray-800'}`}
                                                    title="Table view"
                                                >
                                                    <Table className="w-4 h-4" />
                                                </button>
                                                <div className="w-px h-4 bg-gray-700 mx-2" />
                                                <button
                                                    onClick={(e) => {
                                                        e.preventDefault();
                                                        e.stopPropagation();
                                                        setViewMode('json');
                                                        setTimeout(() => {
                                                            window.scrollTo(window.scrollX, window.scrollY);
                                                        }, 0);
                                                    }}
                                                    className={`p-1 md:p-2 rounded ${viewMode === 'json' ? 'bg-gray-700' : 'hover:bg-gray-800'}`}
                                                    title="JSON view"
                                                >
                                                    <Braces className="w-4 h-4" />
                                                </button>
                                                <div className="w-px h-4 bg-gray-700 mx-2" />
                                                <button
                                                    onClick={() => handleCopyToClipboard(JSON.stringify(resultToShow, null, 2))}
                                                    className="p-2 hover:bg-gray-800 rounded text-white hover:text-gray-200"
                                                    title="Copy result"
                                                >
                                                    <Copy className="w-4 h-4" />
                                                </button>
                                                {shouldShowRollback && (
                                                    !queryStates[queryId]?.isExecuting ? (
                                                        <button
                                                            onClick={(e) => {
                                                                e.preventDefault();
                                                                e.stopPropagation();
                                                                setRollbackState({ show: true, queryId });
                                                                setTimeout(() => {
                                                                    window.scrollTo(window.scrollX, window.scrollY);
                                                                }, 0);
                                                            }}
                                                            className="p-2 hover:bg-gray-800 rounded text-yellow-400 hover:text-yellow-300"
                                                            disabled={queryStates[queryId]?.isExecuting}
                                                        >
                                                            <History className="w-4 h-4" />
                                                        </button>
                                                    ) : (
                                                        <button
                                                            onClick={(e) => {
                                                                e.preventDefault();
                                                                e.stopPropagation();

                                                                // Abort the API call if it's in progress
                                                                if (abortControllerRef.current[queryId]) {
                                                                    abortControllerRef.current[queryId].abort();
                                                                    delete abortControllerRef.current[queryId];
                                                                }

                                                                // Clear any timeouts
                                                                if (queryTimeouts.current[queryId]) {
                                                                    clearTimeout(queryTimeouts.current[queryId]);
                                                                    delete queryTimeouts.current[queryId];
                                                                }

                                                                setRollbackState({ show: false, queryId: null });
                                                                // Update state
                                                                onQueryUpdate(() => {
                                                                    setQueryStates(prev => ({
                                                                        ...prev,
                                                                        [queryId]: { isExecuting: false, isExample: !query.is_executed }
                                                                    }));
                                                                });

                                                                setTimeout(() => {
                                                                    window.scrollTo(window.scrollX, window.scrollY);
                                                                }, 0);
                                                                toast.error('Query cancelled', toastStyle);
                                                            }}
                                                            className="p-2 hover:bg-gray-800 rounded transition-colors text-red-500 hover:text-red-400"
                                                            title="Cancel query"
                                                        >
                                                            <XCircle className="w-4 h-4" />
                                                        </button>
                                                    )
                                                )}
                                            </div>
                                        </div>}
                                    </div>
                                    {query.error ? (
                                        <div className="bg-neo-error/10 text-neo-error p-4 rounded-lg mb-6">
                                            <div className="font-bold mb-2">{query.error.code}</div>
                                            {query.error.message != query.error.details && <div className="mb-2">{query.error.message}</div>}
                                            {query.error.details && (
                                                <div className="text-sm opacity-80 border-t border-neo-error/20 pt-2 mt-2">
                                                    {query.error.details}
                                                </div>
                                            )}
                                        </div>
                                    ) : (
                                        <div className="px-0">
                                            <div className={`
                                            text-green-400 pb-6 w-full
                                                    ${!query.example_result && !query.error ? '' : ''}
                                        `}>
                                                {viewMode === 'table' ? (
                                                    <div className="w-full">
                                                        {shouldShowExampleResult ? (
                                                            resultToShow ? renderTableView(parseResults(resultToShow)) : (
                                                                <div className="text-gray-500">No example data available</div>
                                                            )
                                                        ) : (
                                                            resultToShow ? (
                                                                renderQueryResult(resultToShow)
                                                            ) : (
                                                                <div className="text-gray-500">No data to display</div>
                                                            )
                                                        )}
                                                    </div>
                                                ) : (
                                                    <div className="w-full">
                                                        {shouldShowExampleResult ? (
                                                            resultToShow ? (
                                                                <pre className="overflow-x-auto whitespace-pre-wrap">
                                                                    {JSON.stringify(parseResults(resultToShow), null, 2)}
                                                                </pre>
                                                            ) : (
                                                                <div className="text-gray-500">No example data available</div>
                                                            )
                                                        ) : (
                                                            resultToShow ? (
                                                                renderQueryResult(resultToShow)
                                                            ) : (
                                                                <div className="text-gray-500">No data to display</div>
                                                            )
                                                        )}
                                                    </div>
                                                )}
                                            </div>
                                        </div>
                                    )}
                                </div>
                            )}
                        </div>
                    )}
                </div>
            </div>
        );
    };

    // Add a helper function to remove duplicate queries
    const removeDuplicateQueries = (query: string): string => {
        // Split by semicolon and trim each query
        const queries = query.split(';')
            .map(q => q.trim())
            .filter(q => q.length > 0);

        // Remove duplicates while preserving order
        const uniqueQueries = Array.from(new Set(queries));

        // Join back with semicolons
        return uniqueQueries.join(';\n');
    };

    return (
        <div className={`
                py-4 md:py-6
                ${isFirstMessage ? 'first:pt-0' : ''}
                w-full
              `}>
            <div className={`
        group flex items-center relative
        ${message.type === 'user' ? 'justify-end' : 'justify-start'}
        w-full
      `}>
                {message.type === 'user' && (
                    <div className="
            absolute 
            right-0 
            -bottom-9
            md:-bottom-10 
            flex 
            gap-1
            z-[5]

          ">
                        <button
                            onClick={() => handleCopyToClipboard(message.content)}
                            className="
                -translate-y-1/2
                p-1.5
                md:p-2 
                group-hover:opacity-100 
                transition-colors
                hover:bg-neo-gray
                rounded-lg
                flex-shrink-0
                border-0
                bg-white/80
                backdrop-blur-sm
              "
                            title="Copy message"
                        >
                            <Copy className="w-4 h-4 text-gray-800" />
                        </button>
                        {onEdit && (
                            <button
                                onClick={(e) => {
                                    e.preventDefault();
                                    e.stopPropagation();
                                    onEdit(message.id);
                                    setTimeout(() => {
                                        window.scrollTo(window.scrollX, window.scrollY);
                                    }, 0);
                                }}
                                className="
                  -translate-y-1/2
                  p-1.5
                  md:p-2
                  group-hover:opacity-100 
                  hover:bg-neo-gray
                  transition-colors
                  rounded-lg
                  flex-shrink-0
                  border-0
                  bg-white/80
                  backdrop-blur-sm

                "
                                title="Edit message"
                            >
                                <Pencil className="w-4 h-4 text-gray-800" />
                            </button>
                        )}
                    </div>
                )}
                <div className={`
    message-bubble
    inline-block
        relative
    ${message.type === 'user' ? (
                        editingMessageId === message.id
                            ? 'w-[95%] sm:w-[85%] md:w-[75%]'
                            : 'w-fit max-w-[95%] sm:max-w-[85%] md:max-w-[75%]'
                    ) : 'w-fit max-w-[95%] sm:max-w-[85%] md:max-w-[75%]'}
    ${message.type === 'user'
                        ? 'message-bubble-user'
                        : 'message-bubble-ai'
                    }
`}>
                    <div className={`
        ${editingMessageId === message.id ? 'w-full min-w-full' : 'w-auto min-w-0'}
        ${message.queries?.length ? 'min-w-full' : ''}
    `}>
                        <div className="relative">
                            {message.content.length === 0 && message.loading_steps && message.loading_steps.length > 0 && (
                                <div className={`
                                    ${message.content ? 'animate-fade-up-out absolute w-full' : ''}
                                        text-gray-700
                                    `}>
                                    <LoadingSteps
                                        steps={message.loading_steps.map((step, index) => ({
                                            text: step.text,
                                            done: index !== message.loading_steps!.length - 1
                                        }))}
                                    />
                                </div>
                            )}

                            {editingMessageId === message.id ? (
                                <div className='w-full'>
                                    <textarea
                                        value={editInput}
                                        onChange={(e) => {
                                            e.preventDefault();
                                            e.stopPropagation();
                                            setEditInput(e.target.value);
                                            setTimeout(() => {
                                                window.scrollTo(window.scrollX, window.scrollY);
                                            }, 0);
                                        }}
                                        className="neo-input w-full text-lg min-h-[42px] resize-y py-2 px-3 leading-normal whitespace-pre-wrap"
                                        rows={Math.min(
                                            Math.max(
                                                editInput.split('\n').length,
                                                Math.ceil(editInput.length / 50)
                                            ),
                                            10
                                        )}
                                        autoFocus
                                    />
                                    <div className="flex gap-2 mt-3">
                                        <button
                                            onClick={() => {
                                                onCancelEdit();
                                                setTimeout(() => {
                                                    window.scrollTo(window.scrollX, window.scrollY);
                                                }, 0);
                                            }}
                                            className="neo-button-secondary flex-1 flex items-center justify-center gap-2"
                                        >
                                            <X className="w-4 h-4" />
                                            <span>Cancel</span>
                                        </button>
                                        <button
                                            onClick={() => onSaveEdit(message.id, editInput)}
                                            className="neo-button flex-1 flex items-center justify-center gap-2"
                                        >
                                            <Send className="w-4 h-4" />
                                            <span>Send</span>
                                        </button>
                                    </div>
                                </div>
                            ) : (
                                <div className={message.loading_steps ? 'animate-fade-in' : ''}>
                                    <p className="text-lg whitespace-pre-wrap break-words">
                                        {message.content}
                                        {message.is_edited && message.type === 'user' && (
                                            <span className="ml-2 text-xs text-gray-600 italic">
                                                (edited)
                                            </span>
                                        )}
                                    </p>
                                    {message.queries && message.queries.length > 0 && (
                                        <div className="min-w-full">
                                            {message.queries.map((query: QueryResult, index: number) =>
                                                renderQuery(message.is_streaming || false, query, index)
                                            )}
                                        </div>
                                    )}
                                </div>
                            )}
                        </div>

                        <div className={`
                              text-[12px] text-gray-500 mt-1
                              ${message.type === 'user' ? 'text-right' : 'text-left'}
                            `}>
                            {formatMessageTime(message.created_at)}
                        </div>
                    </div>
                </div>
            </div>

            {rollbackState.show && rollbackState.queryId && (
                <RollbackConfirmationModal
                    onConfirm={() => handleRollback(rollbackState.queryId!)}
                    onCancel={() => setRollbackState({ show: false, queryId: null })}
                />
            )}

            {showCriticalConfirm && (
                <ConfirmationModal
                    title="Critical Query"
                    message="This query may affect important data. Are you sure you want to proceed?"
                    onConfirm={async () => {
                        setShowCriticalConfirm(false);
                        if (queryToExecute !== null) {
                            executeQuery(queryToExecute);
                            setQueryToExecute(null);
                        }
                    }}
                    onCancel={() => {
                        setShowCriticalConfirm(false);
                        setQueryToExecute(null);
                    }}
                />
            )}
        </div>
    );
}