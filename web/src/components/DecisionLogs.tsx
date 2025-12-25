import {
    Brain,
    Check,
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

export default function DecisionLogs({ traderId }: DecisionLogsProps) {
    const { language } = useLanguage()
    const [limit, setLimit] = useState<number>(50)

    const { data: decisions, error, isLoading } = useSWR<DecisionRecord[]>(
        traderId ? `decisions-all-${traderId}-${limit}` : null,
        () => api.getLatestDecisions(traderId, limit),
        {
            refreshInterval: 30000,
            revalidateOnFocus: false,
        }
    )

    if (isLoading) {
        return (
            <div className="binance-card p-6">
                <div className="animate-pulse space-y-4">
                    <div className="h-6 bg-gray-700 rounded w-1/4"></div>
                    <div className="space-y-3">
                        {[1, 2, 3].map((i) => (
                            <div key={i} className="h-32 bg-gray-700 rounded"></div>
                        ))}
                    </div>
                </div>
            </div>
        )
    }

    if (error) {
        return (
            <div className="binance-card p-6">
                <div className="text-center py-8" style={{ color: '#F6465D' }}>
                    {t('errorLoadingDecisions', language)}
                </div>
            </div>
        )
    }

    if (!decisions || decisions.length === 0) {
        return (
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
        )
    }

    return (
        <div className="space-y-6">
            {/* Header with limit selector */}
            <div className="binance-card p-6">
                <div className="flex items-center justify-between">
                    <h2
                        className="text-xl font-bold flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                    >
                        <Brain className="w-5 h-5" style={{ color: '#6366F1' }} />
                        {t('decisionLogs', language)}
                    </h2>
                    <div className="flex items-center gap-2">
                        <span className="text-sm" style={{ color: '#848E9C' }}>
                            {t('showCount', language)}:
                        </span>
                        <select
                            value={limit}
                            onChange={(e) => setLimit(parseInt(e.target.value, 10))}
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

            {/* Decision Logs List - Two columns */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {decisions.map((decision, i) => (
                    <DecisionCard
                        key={i}
                        decision={decision}
                        language={language}
                    />
                ))}
            </div>
        </div>
    )
}

// Decision Card Component
function DecisionCard({
    decision,
    language,
}: {
    decision: DecisionRecord
    language: Language
}) {
    const [showInputPrompt, setShowInputPrompt] = useState(false)
    const [showCoT, setShowCoT] = useState(false)
    const [showExecutionLog, setShowExecutionLog] = useState(false)

    return (
        <div
            className="binance-card p-6 transition-all duration-300 hover:translate-y-[-2px]"
            style={{
                boxShadow: '0 2px 8px rgba(0, 0, 0, 0.3)',
            }}
        >
            {/* Header */}
            <div className="flex items-start justify-between mb-4">
                <div>
                    <div className="font-semibold text-lg" style={{ color: '#EAECEF' }}>
                        {t('cycle', language)} #{decision.cycle_number}
                    </div>
                    <div className="text-sm mt-1" style={{ color: '#848E9C' }}>
                        {new Date(decision.timestamp).toLocaleString()}
                    </div>
                </div>
                <div
                    className="px-3 py-1 rounded text-xs font-bold"
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
                <div className="mb-4">
                    <button
                        onClick={() => setShowInputPrompt(!showInputPrompt)}
                        className="flex items-center gap-2 text-sm transition-colors w-full text-left"
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
                            className="mt-2 rounded p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
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
                <div className="mb-4">
                    <button
                        onClick={() => setShowCoT(!showCoT)}
                        className="flex items-center gap-2 text-sm transition-colors w-full text-left"
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
                            className="mt-2 rounded p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
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
                <div className="space-y-2 mb-4">
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
                            <div className="flex items-center gap-2 text-sm flex-wrap">
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
                    className="flex flex-wrap gap-4 text-xs mb-4 rounded px-3 py-2"
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
                <div className="mb-4">
                    <button
                        onClick={() => setShowExecutionLog(!showExecutionLog)}
                        className="flex items-center gap-2 text-sm transition-colors w-full text-left"
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
                            className="mt-2 rounded p-4 text-sm font-mono space-y-1 max-h-96 overflow-y-auto"
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
                    className="text-sm rounded px-3 py-2 mt-4 flex items-center gap-2"
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

