import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'

export default {
  operator: {
    common: { loading: '加载中…', refresh: '刷新', retry: '重试' },
    nav: { overview: '概况', users: '用户管理' },
    overview: { title: '运营概况', refresh: '刷新', groups: '分组', channels: '渠道状态', latency: '毫秒', groupsEmpty: '暂无分组', channelsEmpty: '暂无渠道状态', loadError: '运营数据加载失败', modelCount: '{count} 个模型' },
    users: {
      statuses: { active: '启用', disabled: '停用' },
      title: '用户管理', search: '搜索用户', searchPlaceholder: '搜索用户', user: '用户', status: '状态', balance: '余额', details: '详情', empty: '未找到用户', loadError: '用户加载失败', detailLoadError: '该分区加载失败', detailTabs: '用户详情', totalBalance: '总余额', freeBalance: '免费余额', paidBalance: '付费余额',
      tabs: { ledger: '余额流水', orders: '支付订单', usage: '用量', adjust: '余额调整' }, ledgerEmpty: '暂无余额流水', ordersEmpty: '暂无支付订单', usageEmpty: '暂无用量数据', usagePeriod: '用量周期', periods: { today: '今天', '7d': '近 7 天', '30d': '近 30 天', '90d': '近 90 天' }, usageSummary: '{period}：{requests} 次请求，{tokens} tokens，金额 {amount}（{start} – {end}）',
      operation: '操作', adjustAdd: '增加', adjustSubtract: '扣减', source: '余额来源', amount: '金额', amountPlaceholder: '请输入金额', reason: '原因', reasonPlaceholder: '请输入原因', invalidAdjustment: '请输入有效金额和原因', adjustmentFailed: '余额调整失败', submitAdjustment: '提交调整', retryAdjustment: '重试调整', retryPending: '使用相同凭证重试待处理调整', cachePending: '余额已记账，但缓存同步尚未完成；重试会使用相同凭证。', adjustmentResult: '余额已从 {before} 变为 {after}。', resolvePending: '请先处理待确认的调整，再发起新的调整。',
      recoveredPending: '此前的调整可能已到达服务器。浏览器未保存请求内容，请先核对用户 #{target} 的余额流水，再允许新的调整。', acknowledgeLedger: '我已核对流水', acknowledgeLedgerConfirm: '确认已核对流水并知悉新的调整将使用新的幂等凭证。', pendingStorageUnavailable: '安全的待处理调整记录无法读取或写入，已阻断新的调整。', authorizationChanged: '您的运营员授权已变化，已阻断新的调整。', leavePendingWarning: '当前调整可能已经提交。离开会丢失内存中的重试内容；再次调整前请先核对流水。'
    }
  },
  ...landing,
  ...common,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  admin,
  ...misc,
}
