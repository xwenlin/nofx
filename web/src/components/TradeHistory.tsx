import { ChevronLeft, ChevronRight, TrendingDown, TrendingUp } from 'lucide-react'
import { useState } from 'react'
import useSWR from 'swr'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { api } from '../lib/api'

interface TradeHistoryProps {
    traderId: string
}

export default function TradeHistory({ traderId }: TradeHistoryProps) {
    const { language } = useLanguage()
    const [page, setPage] = useState<number>(1)
    const [pageSize, setPageSize] = useState<number>(50)

    const { data: response, error, isLoading } = useSWR(
        traderId ? `trades-${traderId}-${page}-${pageSize}` : null,
        () => api.getTrades(traderId, page, pageSize),
        {
            refreshInterval: 30000,
            revalidateOnFocus: false,
        }
    )

    const trades = response?.data || []
    const total = response?.total || 0
    const totalPages = response?.total_pages || 0

    if (isLoading) {
        return (
            <div className="binance-card p-6">
                <div className="animate-pulse space-y-4">
                    <div className="h-6 bg-gray-700 rounded w-1/4"></div>
                    <div className="h-64 bg-gray-700 rounded"></div>
                </div>
            </div>
        )
    }

    if (error) {
        return (
            <div className="binance-card p-6">
                <div className="text-center py-8" style={{ color: '#F6465D' }}>
                    {t('errorLoadingTrades', language)}
                </div>
            </div>
        )
    }

    if (!trades || trades.length === 0) {
        return (
            <div className="binance-card p-6">
                <div className="text-center py-16" style={{ color: '#848E9C' }}>
                    <div className="mb-4 opacity-50 flex justify-center">
                        <TrendingUp className="w-16 h-16" />
                    </div>
                    <div className="text-lg font-semibold mb-2">
                        {t('noTradesYet', language)}
                    </div>
                    <div className="text-sm">
                        {t('tradesWillAppearHere', language)}
                    </div>
                </div>
            </div>
        )
    }

    return (
        <div className="space-y-6">
            {/* Header with pagination controls */}
            <div className="binance-card p-6">
                <div className="flex items-center justify-between mb-6 flex-wrap gap-4">
                    <h2
                        className="text-xl font-bold flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                    >
                        <TrendingUp className="w-5 h-5" style={{ color: '#F0B90B' }} />
                        {t('tradeHistory', language)}
                        {total > 0 && (
                            <span className="text-sm font-normal ml-2" style={{ color: '#848E9C' }}>
                                ({total} {t('total', language)})
                            </span>
                        )}
                    </h2>
                    <div className="flex items-center gap-4 flex-wrap">
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

            {/* Table */}
            <div className="binance-card p-6">
                <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                        <thead className="text-left border-b" style={{ borderColor: '#2B3139' }}>
                            <tr>
                                <th className="pb-3 font-semibold" style={{ color: '#848E9C' }}>
                                    {t('time', language)}
                                </th>
                                <th className="pb-3 font-semibold" style={{ color: '#848E9C' }}>
                                    {t('symbol', language)}
                                </th>
                                <th className="pb-3 font-semibold" style={{ color: '#848E9C' }}>
                                    {t('side', language)}
                                </th>
                                <th className="pb-3 font-semibold" style={{ color: '#848E9C' }}>
                                    {t('openPrice', language)}
                                </th>
                                <th className="pb-3 font-semibold" style={{ color: '#848E9C' }}>
                                    {t('closePrice', language)}
                                </th>
                                <th className="pb-3 font-semibold" style={{ color: '#848E9C' }}>
                                    {t('quantity', language)}
                                </th>
                                <th className="pb-3 font-semibold" style={{ color: '#848E9C' }}>
                                    {t('leverage', language)}
                                </th>
                                <th className="pb-3 font-semibold" style={{ color: '#848E9C' }}>
                                    {t('pnl', language)}
                                </th>
                                <th className="pb-3 font-semibold" style={{ color: '#848E9C' }}>
                                    {t('closeReason', language)}
                                </th>
                            </tr>
                        </thead>
                        <tbody>
                            {trades.map((trade) => {
                                const isClosed = trade.close_time !== null
                                const isProfitable = trade.pnl !== null && trade.pnl > 0
                                const isLong = trade.side === 'long'

                                return (
                                    <tr
                                        key={trade.id}
                                        className="border-b last:border-0"
                                        style={{ borderColor: '#2B3139' }}
                                    >
                                        <td className="py-3" style={{ color: '#848E9C' }}>
                                            {isClosed
                                                ? new Date(trade.close_time!).toLocaleString()
                                                : new Date(trade.open_time).toLocaleString()}
                                        </td>
                                        <td className="py-3 font-mono font-semibold">
                                            {trade.symbol}
                                        </td>
                                        <td className="py-3">
                                            <span
                                                className="px-2 py-1 rounded text-xs font-bold"
                                                style={
                                                    isLong
                                                        ? {
                                                            background: 'rgba(14, 203, 129, 0.1)',
                                                            color: '#0ECB81',
                                                        }
                                                        : {
                                                            background: 'rgba(246, 70, 93, 0.1)',
                                                            color: '#F6465D',
                                                        }
                                                }
                                            >
                                                {isLong
                                                    ? t('long', language).toUpperCase()
                                                    : t('short', language).toUpperCase()}
                                            </span>
                                        </td>
                                        <td
                                            className="py-3 font-mono"
                                            style={{ color: '#EAECEF' }}
                                        >
                                            {trade.open_price.toFixed(4)}
                                        </td>
                                        <td
                                            className="py-3 font-mono"
                                            style={{ color: '#EAECEF' }}
                                        >
                                            {trade.close_price
                                                ? trade.close_price.toFixed(4)
                                                : '-'}
                                        </td>
                                        <td
                                            className="py-3 font-mono"
                                            style={{ color: '#EAECEF' }}
                                        >
                                            {trade.quantity.toFixed(4)}
                                        </td>
                                        <td
                                            className="py-3 font-mono"
                                            style={{ color: '#F0B90B' }}
                                        >
                                            {trade.leverage}x
                                        </td>
                                        <td className="py-3 font-mono font-bold">
                                            {isClosed ? (
                                                <span
                                                    style={{
                                                        color: isProfitable ? '#0ECB81' : '#F6465D',
                                                    }}
                                                >
                                                    {isProfitable ? (
                                                        <TrendingUp className="w-4 h-4 inline mr-1" />
                                                    ) : (
                                                        <TrendingDown className="w-4 h-4 inline mr-1" />
                                                    )}
                                                    {trade.pnl !== null
                                                        ? `${trade.pnl >= 0 ? '+' : ''}${trade.pnl.toFixed(2)} USDT`
                                                        : '-'}
                                                    {trade.pnl_pct !== null && (
                                                        <span className="ml-2">
                                                            ({trade.pnl_pct >= 0 ? '+' : ''}
                                                            {trade.pnl_pct.toFixed(2)}%)
                                                        </span>
                                                    )}
                                                </span>
                                            ) : (
                                                <span style={{ color: '#848E9C' }}>
                                                    {t('open', language)}
                                                </span>
                                            )}
                                        </td>
                                        <td className="py-3">
                                            {trade.close_reason ? (
                                                <span
                                                    className="px-2 py-1 rounded text-xs font-semibold"
                                                    style={{
                                                        background:
                                                            trade.close_reason === 'stop_loss'
                                                                ? 'rgba(246, 70, 93, 0.1)'
                                                                : trade.close_reason === 'take_profit'
                                                                    ? 'rgba(14, 203, 129, 0.1)'
                                                                    : 'rgba(96, 165, 250, 0.1)',
                                                        color:
                                                            trade.close_reason === 'stop_loss'
                                                                ? '#F6465D'
                                                                : trade.close_reason === 'take_profit'
                                                                    ? '#0ECB81'
                                                                    : '#60a5fa',
                                                    }}
                                                >
                                                    {trade.close_reason === 'stop_loss'
                                                        ? t('stopLoss', language)
                                                        : trade.close_reason === 'take_profit'
                                                            ? t('takeProfit', language)
                                                            : trade.close_reason === 'manual'
                                                                ? t('manual', language)
                                                                : trade.close_reason}
                                                </span>
                                            ) : (
                                                <span style={{ color: '#848E9C' }}>-</span>
                                            )}
                                        </td>
                                    </tr>
                                )
                            })}
                        </tbody>
                    </table>
                </div>
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
