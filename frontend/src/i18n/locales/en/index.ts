import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'

export default {
  operator: {
    common: { loading: 'Loading…', refresh: 'Refresh', retry: 'Retry' },
    nav: { overview: 'Overview', users: 'Users' },
    overview: { title: 'Operator Overview', refresh: 'Refresh', groups: 'Groups', channels: 'Channel Status', latency: 'ms', groupsEmpty: 'No groups', channelsEmpty: 'No channel status', loadError: 'Unable to load operator data', modelCount: '{count} models' },
    users: {
      statuses: { active: 'Active', disabled: 'Disabled' },
      title: 'Operator Users', search: 'Search users', searchPlaceholder: 'Search users', user: 'User', status: 'Status', balance: 'Balance', details: 'Details', empty: 'No users found', loadError: 'Unable to load users', detailLoadError: 'Unable to load this section', detailTabs: 'User details', totalBalance: 'Total balance', freeBalance: 'Free balance', paidBalance: 'Paid balance',
      tabs: { ledger: 'Balance history', orders: 'Payment orders', usage: 'Usage', adjust: 'Adjust balance' }, ledgerEmpty: 'No balance history', ordersEmpty: 'No payment orders', usageEmpty: 'No usage data', usagePeriod: 'Usage period', periods: { today: 'Today', '7d': 'Last 7 days', '30d': 'Last 30 days', '90d': 'Last 90 days' }, usageSummary: '{period}: {requests} requests, {tokens} tokens, {amount} used ({start} – {end})',
      operation: 'Operation', adjustAdd: 'Add', adjustSubtract: 'Subtract', source: 'Balance source', amount: 'Amount', amountPlaceholder: 'Enter an amount', reason: 'Reason', reasonPlaceholder: 'Enter a reason', invalidAdjustment: 'Enter a valid amount and reason', adjustmentFailed: 'Balance adjustment failed', submitAdjustment: 'Submit adjustment', retryAdjustment: 'Retry adjustment', retryPending: 'Retry the pending adjustment with the same credential', cachePending: 'Balance was recorded; cache synchronization is pending. Retry uses the same credential.', adjustmentResult: 'Balance changed from {before} to {after}.', resolvePending: 'Resolve the pending adjustment before starting another one.',
      recoveredPending: 'A previous adjustment may have reached the server. Its body is not stored in this browser. Check user #{target}’s ledger before allowing another adjustment.', acknowledgeLedger: 'I checked the ledger', acknowledgeLedgerConfirm: 'Confirm that you checked the ledger and understand a new adjustment will use a new credential.', pendingStorageUnavailable: 'The safe pending-adjustment record cannot be read or written. New adjustments are blocked.', authorizationChanged: 'Your operator authorization changed. New adjustments are blocked.', leavePendingWarning: 'This adjustment may already have been submitted. Leaving loses the in-memory retry body. Check the ledger before another adjustment.'
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
