import {
    Brain,
    Check,
    ChevronLeft,
    ChevronRight,
    ChevronRight as ChevronRightIcon,
    Filter,
    Inbox,
    Send,
    X,
    XCircle
} from 'lucide-react'
import { useState } from 'react'
import useSWR from 'swr'
import { useLanguage } from '../contexts/LanguageContext'
import { t, type Language } from '../i18n/translations'
import { api } from '../lib/api'
import { stripLeadingIcons } from '../lib/text'
import type { DecisionRecord } from '../types'

interface DecisionLogsProps {
    traderId: string
}

type ActionFilter = 'all' | 'has_trading' | 'wait_only' | 'open_only' | 'close_only'
type StatusFilter = 'all' | 'decision_failed' | 'action_failed'

export default function DecisionLogs({ traderId }: DecisionLogsProps) {
    const { language } = useLanguage()
    const [page, setPage] = useState<number>(1)
    const [pageSize, setPageSize] = useState<number>(50)
    const [actionFilter, setActionFilter] = useState<ActionFilter>('all')
    const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
    const [selectedDecision, setSelectedDecision] = useState<DecisionRecord | null>(null)

    const { data: response, error, isLoading } = useSWR(
        traderId ? `decisions-${traderId}-${page}-${pageSize}-${actionFilter}-${statusFilter}` : null,
        () => api.getDecisions(traderId, page, pageSize, actionFilter, statusFilter),
        {
            refreshInterval: 30000,
            revalidateOnFocus: false,
        }
    )

    const decisions = response?.data || []
    const total = response?.total || 0
    const totalPages = response?.total_pages || 0

    // 渲染过滤条件头部（始终显示）
    const renderHeader = () => (
        <div className="binance-card p-6">
            <div className="flex items-center justify-between flex-wrap gap-4">
                <h2
                    className="text-xl font-bold flex items-center gap-2"
                    style={{ color: '#EAECEF' }}
                >
                    <Brain className="w-5 h-5" style={{ color: '#6366F1' }} />
                    {t('decisionLogs', language)}
                    {total > 0 && (
                        <span className="text-sm font-normal ml-2" style={{ color: '#848E9C' }}>
                            ({total} {t('total', language)})
                        </span>
                    )}
                </h2>
                <div className="flex items-center gap-4 flex-wrap">
                    <div className="flex items-center gap-2">
                        <Filter className="w-4 h-4" style={{ color: '#848E9C' }} />
                        <span className="text-sm" style={{ color: '#848E9C' }}>
                            {t('filterByAction', language)}:
                        </span>
                        <select
                            value={actionFilter}
                            onChange={(e) => {
                                setActionFilter(e.target.value as ActionFilter)
                                setPage(1) // 重置到第一页
                            }}
                            className="rounded px-3 py-2 text-sm font-medium cursor-pointer transition-colors"
                            style={{
                                background: '#1E2329',
                                border: '1px solid #2B3139',
                                color: '#EAECEF',
                            }}
                        >
                            <option value="all">{t('filterAll', language)}</option>
                            <option value="has_trading">{t('filterHasTrading', language)}</option>
                            <option value="wait_only">{t('filterWaitOnly', language)}</option>
                            <option value="open_only">{t('filterOpenOnly', language)}</option>
                            <option value="close_only">{t('filterCloseOnly', language)}</option>
                        </select>
                    </div>
                    <div className="flex items-center gap-2">
                        <span className="text-sm" style={{ color: '#848E9C' }}>
                            {t('filterByStatus', language)}:
                        </span>
                        <select
                            value={statusFilter}
                            onChange={(e) => {
                                setStatusFilter(e.target.value as StatusFilter)
                                setPage(1) // 重置到第一页
                            }}
                            className="rounded px-3 py-2 text-sm font-medium cursor-pointer transition-colors"
                            style={{
                                background: '#1E2329',
                                border: '1px solid #2B3139',
                                color: '#EAECEF',
                            }}
                        >
                            <option value="all">{t('filterAll', language)}</option>
                            <option value="decision_failed">{t('filterDecisionFailed', language)}</option>
                            <option value="action_failed">{t('filterActionFailed', language)}</option>
                        </select>
                    </div>
                    <div className="flex items-center gap-2">
                        <span className="text-sm" style={{ color: '#848E9C' }}>
                            {t('pageSize', language)}:
                        </span>
                        <select
                            value={pageSize}
                            onChange={(e) => {
                                setPageSize(parseInt(e.target.value, 10))
                                setPage(1) // 重置到第一页
                            }}
                            className="rounded px-3 py-2 text-sm font-medium cursor-pointer transition-colors"
                            style={{
                                background: '#1E2329',
                                border: '1px solid #2B3139',
                                color: '#EAECEF',
                            }}
                        >
                            <option value={20}>20</option>
                            <option value={50}>50</option>
                            <option value={100}>100</option>
                            <option value={200}>200</option>
                        </select>
                    </div>
                </div>
            </div>
        </div>
    )

    if (isLoading) {
        return (
            <div className="space-y-6">
                {renderHeader()}
                <div className="binance-card p-6">
                    <div className="animate-pulse space-y-4">
                        <div className="h-6 bg-gray-700 rounded w-1/4"></div>
                        <div className="space-y-3">
                            {[1, 2, 3, 4, 5].map((i) => (
                                <div key={i} className="h-12 bg-gray-700 rounded"></div>
                            ))}
                        </div>
                    </div>
                </div>
            </div>
        )
    }

    if (error) {
        return (
            <div className="space-y-6">
                {renderHeader()}
                <div className="binance-card p-6">
                    <div className="text-center py-8" style={{ color: '#F6465D' }}>
                        {t('errorLoadingDecisions', language)}
                    </div>
                </div>
            </div>
        )
    }

    const hasNoData = !decisions || decisions.length === 0
    const hasFilteredResults = hasNoData && actionFilter !== 'all'

    if (hasNoData && !hasFilteredResults) {
        return (
            <div className="space-y-6">
                {renderHeader()}
                <div className="binance-card p-6">
                    <div className="text-center py-16" style={{ color: '#848E9C' }}>
                        <div className="mb-4 opacity-50 flex justify-center">
                            <Brain className="w-16 h-16" />
                        </div>
                        <div className="text-lg font-semibold mb-2">
                            {t('noDecisionsYet', language)}
                        </div>
                        <div className="text-sm">
                            {t('aiDecisionsWillAppear', language)}
                        </div>
                    </div>
                </div>
            </div>
        )
    }

    if (hasFilteredResults) {
        return (
            <div className="space-y-6">
                {renderHeader()}
                <div className="binance-card p-6">
                    <div className="text-center py-16" style={{ color: '#848E9C' }}>
                        <div className="mb-4 opacity-50 flex justify-center">
                            <Filter className="w-16 h-16" />
                        </div>
                        <div className="text-lg font-semibold mb-2">
                            {t('noFilteredDecisions', language)}
                        </div>
                        <div className="text-sm">
                            {t('tryDifferentFilter', language)}
                        </div>
                    </div>
                </div>
            </div>
        )
    }

    // 获取动作摘要
    const getActionSummary = (decision: DecisionRecord): string => {
        if (!decision.decisions || decision.decisions.length === 0) {
            return '-'
        }
        const actions = decision.decisions.map(a => a.action).filter(Boolean)
        if (actions.length === 0) return '-'
        return actions.slice(0, 3).join(', ') + (actions.length > 3 ? '...' : '')
    }

    // 获取成功动作数量
    const getSuccessCount = (decision: DecisionRecord): number => {
        if (!decision.decisions) return 0
        return decision.decisions.filter(a => a.success).length
    }

    // 获取总动作数量
    const getTotalActions = (decision: DecisionRecord): number => {
        return decision.decisions?.length || 0
    }

    return (
        <div className="space-y-6">
            {renderHeader()}

            {/* 表格 + 侧边栏布局 */}
            <div className="flex gap-6">
                {/* 左侧：表格列表 */}
                <div className={`flex-1 transition-all duration-300 ${selectedDecision ? 'mr-0' : ''}`}>
                    <div className="binance-card overflow-hidden">
                        <div className="overflow-x-auto">
                            <table className="w-full" style={{ borderCollapse: 'separate', borderSpacing: 0 }}>
                                <thead>
                                    <tr style={{ background: '#1E2329' }}>
                                        <th
                                            className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider"
                                            style={{ color: '#848E9C', borderBottom: '1px solid #2B3139' }}
                                        >
                                            {t('cycle', language)}
                                        </th>
                                        <th
                                            className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider"
                                            style={{ color: '#848E9C', borderBottom: '1px solid #2B3139' }}
                                        >
                                            {t('time', language)}
                                        </th>
                                        <th
                                            className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider"
                                            style={{ color: '#848E9C', borderBottom: '1px solid #2B3139' }}
                                        >
                                            {t('status', language)}
                                        </th>
                                        <th
                                            className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider"
                                            style={{ color: '#848E9C', borderBottom: '1px solid #2B3139' }}
                                        >
                                            {t('actions', language)}
                                        </th>
                                        <th
                                            className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider"
                                            style={{ color: '#848E9C', borderBottom: '1px solid #2B3139' }}
                                        >
                                            {t('successRate', language)}
                                        </th>
                                        <th
                                            className="px-4 py-3 text-center text-xs font-semibold uppercase tracking-wider"
                                            style={{ color: '#848E9C', borderBottom: '1px solid #2B3139', width: '60px' }}
                                        >
                                            {t('details', language)}
                                        </th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {decisions.map((decision, i) => {
                                        const isSelected = selectedDecision?.cycle_number === decision.cycle_number &&
                                            selectedDecision?.timestamp === decision.timestamp
                                        const successCount = getSuccessCount(decision)
                                        const totalActions = getTotalActions(decision)
                                        const successRate = totalActions > 0
                                            ? `${successCount}/${totalActions}`
                                            : '-'

                                        return (
                                            <tr
                                                key={i}
                                                onClick={() => setSelectedDecision(decision)}
                                                className="cursor-pointer transition-colors hover:bg-opacity-50"
                                                style={{
                                                    background: isSelected ? 'rgba(99, 102, 241, 0.1)' : 'transparent',
                                                    borderBottom: i < decisions.length - 1 ? '1px solid #2B3139' : 'none',
                                                }}
                                            >
                                                <td className="px-4 py-3">
                                                    <span className="font-semibold" style={{ color: '#EAECEF' }}>
                                                        #{decision.cycle_number}
                                                    </span>
                                                </td>
                                                <td className="px-4 py-3">
                                                    <span className="text-sm" style={{ color: '#848E9C' }}>
                                                        {new Date(decision.timestamp).toLocaleString()}
                                                    </span>
                                                </td>
                                                <td className="px-4 py-3">
                                                    <span
                                                        className="px-2 py-1 rounded text-xs font-bold"
                                                        style={
                                                            decision.success
                                                                ? { background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81' }
                                                                : { background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }
                                                        }
                                                    >
                                                        {t(decision.success ? 'success' : 'failed', language)}
                                                    </span>
                                                </td>
                                                <td className="px-4 py-3">
                                                    <span className="text-sm" style={{ color: '#EAECEF' }}>
                                                        {getActionSummary(decision)}
                                                    </span>
                                                </td>
                                                <td className="px-4 py-3">
                                                    <span className="text-sm font-mono" style={{ color: '#848E9C' }}>
                                                        {successRate}
                                                    </span>
                                                </td>
                                                <td className="px-4 py-3 text-center">
                                                    <ChevronRightIcon
                                                        className={`w-4 h-4 inline transition-transform ${isSelected ? 'rotate-90' : ''}`}
                                                        style={{ color: '#848E9C' }}
                                                    />
                                                </td>
                                            </tr>
                                        )
                                    })}
                                </tbody>
                            </table>
                        </div>
                    </div>
                </div>

                {/* 右侧：详情侧边栏 */}
                {selectedDecision && (
                    <div className="w-[768px] flex-shrink-0">
                        <DecisionSidebar
                            decision={selectedDecision}
                            language={language}
                            onClose={() => setSelectedDecision(null)}
                        />
                    </div>
                )}
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
                <div className="binance-card p-6">
                    <div className="flex items-center justify-between">
                        <div className="text-sm" style={{ color: '#848E9C' }}>
                            {t('page', language)} {page} / {totalPages} ({total} {t('total', language)})
                        </div>
                        <div className="flex items-center gap-2">
                            <button
                                onClick={() => setPage(Math.max(1, page - 1))}
                                disabled={page === 1}
                                className="px-3 py-2 rounded text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                                style={{
                                    background: page === 1 ? '#1E2329' : '#2B3139',
                                    border: '1px solid #2B3139',
                                    color: '#EAECEF',
                                }}
                            >
                                <ChevronLeft className="w-4 h-4 inline" /> {t('previous', language)}
                            </button>
                            <button
                                onClick={() => setPage(Math.min(totalPages, page + 1))}
                                disabled={page === totalPages}
                                className="px-3 py-2 rounded text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                                style={{
                                    background: page === totalPages ? '#1E2329' : '#2B3139',
                                    border: '1px solid #2B3139',
                                    color: '#EAECEF',
                                }}
                            >
                                {t('next', language)} <ChevronRight className="w-4 h-4 inline" />
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}

// 侧边栏详情组件
function DecisionSidebar({
    decision,
    language,
    onClose,
}: {
    decision: DecisionRecord
    language: Language
    onClose: () => void
}) {
    const [showInputPrompt, setShowInputPrompt] = useState(false)
    const [showCoT, setShowCoT] = useState(false)
    const [showExecutionLog, setShowExecutionLog] = useState(false)

    return (
        <div
            className="binance-card p-6 h-full overflow-y-auto"
            style={{ maxHeight: 'calc(100vh - 300px)' }}
        >
            {/* Header */}
            <div className="flex items-start justify-between mb-6">
                <div className="flex-1">
                    <div className="font-semibold text-lg mb-1" style={{ color: '#EAECEF' }}>
                        {t('cycle', language)} #{decision.cycle_number}
                    </div>
                    <div className="text-sm" style={{ color: '#848E9C' }}>
                        {new Date(decision.timestamp).toLocaleString()}
                    </div>
                </div>
                <button
                    onClick={onClose}
                    className="ml-4 p-1 rounded hover:bg-opacity-50 transition-colors"
                    style={{ color: '#848E9C' }}
                >
                    <X className="w-5 h-5" />
                </button>
            </div>

            {/* Status Badge */}
            <div className="mb-6">
                <div
                    className="px-3 py-1 rounded text-xs font-bold inline-block"
                    style={
                        decision.success
                            ? { background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81' }
                            : { background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }
                    }
                >
                    {t(decision.success ? 'success' : 'failed', language)}
                </div>
            </div>

            {/* Input Prompt - Collapsible */}
            {decision.input_prompt && (
                <div className="mb-6">
                    <button
                        onClick={() => setShowInputPrompt(!showInputPrompt)}
                        className="flex items-center gap-2 text-sm transition-colors w-full text-left mb-2"
                        style={{ color: '#60a5fa' }}
                    >
                        <span className="font-semibold flex items-center gap-2">
                            <Inbox className="w-4 h-4" /> {t('inputPrompt', language)}
                        </span>
                        <span className="text-xs ml-auto">
                            {showInputPrompt
                                ? t('collapse', language)
                                : t('expand', language)}
                        </span>
                    </button>
                    {showInputPrompt && (
                        <div
                            className="rounded p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
                            style={{
                                background: '#0B0E11',
                                border: '1px solid #2B3139',
                                color: '#EAECEF',
                            }}
                        >
                            {decision.input_prompt}
                        </div>
                    )}
                </div>
            )}

            {/* AI Chain of Thought - Collapsible */}
            {decision.cot_trace && (
                <div className="mb-6">
                    <button
                        onClick={() => setShowCoT(!showCoT)}
                        className="flex items-center gap-2 text-sm transition-colors w-full text-left mb-2"
                        style={{ color: '#F0B90B' }}
                    >
                        <span className="font-semibold flex items-center gap-2">
                            <Send className="w-4 h-4" />{' '}
                            {stripLeadingIcons(t('aiThinking', language))}
                        </span>
                        <span className="text-xs ml-auto">
                            {showCoT ? t('collapse', language) : t('expand', language)}
                        </span>
                    </button>
                    {showCoT && (
                        <div
                            className="rounded p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
                            style={{
                                background: '#0B0E11',
                                border: '1px solid #2B3139',
                                color: '#EAECEF',
                            }}
                        >
                            {decision.cot_trace}
                        </div>
                    )}
                </div>
            )}

            {/* Decisions Actions */}
            {decision.decisions && decision.decisions.length > 0 && (
                <div className="space-y-3 mb-6">
                    <div className="text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
                        {t('actions', language)}:
                    </div>
                    {decision.decisions.map((action, j) => (
                        <div
                            key={j}
                            className="rounded px-3 py-2"
                            style={{ background: '#0B0E11' }}
                        >
                            {/* Action Header */}
                            <div className="flex items-center gap-2 text-sm flex-wrap mb-2">
                                <span
                                    className="font-mono font-bold"
                                    style={{ color: '#EAECEF' }}
                                >
                                    {action.symbol}
                                </span>
                                <span
                                    className="px-2 py-0.5 rounded text-xs font-bold"
                                    style={
                                        action.action.includes('open')
                                            ? {
                                                background: 'rgba(96, 165, 250, 0.1)',
                                                color: '#60a5fa',
                                            }
                                            : {
                                                background: 'rgba(240, 185, 11, 0.1)',
                                                color: '#F0B90B',
                                            }
                                    }
                                >
                                    {action.action}
                                </span>
                                {action.leverage > 0 && (
                                    <span style={{ color: '#F0B90B' }}>{action.leverage}x</span>
                                )}
                                {action.price > 0 && (
                                    <span
                                        className="font-mono text-xs"
                                        style={{ color: '#848E9C' }}
                                    >
                                        @{action.price.toFixed(4)}
                                    </span>
                                )}
                                <span style={{ color: action.success ? '#0ECB81' : '#F6465D' }}>
                                    {action.success ? (
                                        <Check className="w-3 h-3 inline" />
                                    ) : (
                                        <X className="w-3 h-3 inline" />
                                    )}
                                </span>
                                {action.error && (
                                    <span className="text-xs ml-2" style={{ color: '#F6465D' }}>
                                        {action.error}
                                    </span>
                                )}
                            </div>
                            {/* Reasoning */}
                            {action.reasoning && (
                                <div className="mt-2 text-xs rounded px-2 py-1.5 whitespace-pre-wrap" style={{
                                    background: '#1E2329',
                                    border: '1px solid #2B3139',
                                    color: '#EAECEF',
                                }}>
                                    {action.reasoning}
                                </div>
                            )}
                        </div>
                    ))}
                </div>
            )}

            {/* Account State Summary */}
            {decision.account_state && (
                <div
                    className="flex flex-wrap gap-4 text-xs mb-6 rounded px-3 py-2"
                    style={{ background: '#0B0E11', color: '#848E9C' }}
                >
                    <span>
                        {t('totalBalance', language)}:{' '}
                        {decision.account_state.total_balance.toFixed(2)} USDT
                    </span>
                    <span>
                        {t('availableBalance', language)}:{' '}
                        {decision.account_state.available_balance.toFixed(2)} USDT
                    </span>
                    <span>
                        {t('marginUsedPct', language)}:{' '}
                        {decision.account_state.margin_used_pct.toFixed(1)}%
                    </span>
                    <span>
                        {t('positions', language)}: {decision.account_state.position_count}
                    </span>
                </div>
            )}

            {/* Execution Logs - Collapsible */}
            {decision.execution_log && decision.execution_log.length > 0 && (
                <div className="mb-6">
                    <button
                        onClick={() => setShowExecutionLog(!showExecutionLog)}
                        className="flex items-center gap-2 text-sm transition-colors w-full text-left mb-2"
                        style={{ color: '#848E9C' }}
                    >
                        <span className="font-semibold">
                            {t('executionLog', language)} (
                            {decision.execution_log.length})
                        </span>
                        <span className="text-xs ml-auto">
                            {showExecutionLog
                                ? t('collapse', language)
                                : t('expand', language)}
                        </span>
                    </button>
                    {showExecutionLog && (
                        <div
                            className="rounded p-4 text-sm font-mono space-y-1 max-h-96 overflow-y-auto"
                            style={{
                                background: '#0B0E11',
                                border: '1px solid #2B3139',
                                color: '#EAECEF',
                            }}
                        >
                            {decision.execution_log.map((log, k) => (
                                <div
                                    key={k}
                                    className="text-xs"
                                    style={{
                                        color:
                                            log.includes('✓') || log.includes('成功')
                                                ? '#0ECB81'
                                                : '#F6465D',
                                    }}
                                >
                                    {log}
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            )}

            {/* Error Message */}
            {decision.error_message && (
                <div
                    className="text-sm rounded px-3 py-2 flex items-center gap-2"
                    style={{
                        color: '#F6465D',
                        background: 'rgba(246, 70, 93, 0.1)',
                    }}
                >
                    <XCircle className="w-4 h-4" /> {decision.error_message}
                </div>
            )}
        </div>
    )
}
