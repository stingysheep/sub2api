"""Extract only usage/model metadata for this task tree; never copy rollout content."""
import datetime
import json
import os
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / '.local' / 'autonomous-backlog'
REPORT = ROOT / 'notes' / 'model-routing-efficiency.md'
START = '2026-09-15T17:31:20.352Z'
FIELDS = ['input_tokens', 'cached_input_tokens', 'output_tokens', 'reasoning_output_tokens', 'total_tokens']
ASSIGNMENTS = {
    '/root': ['协调/独立复核/环境/审计', 'BUG-07', 'BUG-08', 'BUG-09', 'PERF-03', 'TEST-01', 'OPS-02/03/05返工'],
    '/root/scheduling': ['BUG-01', 'TEST-03'],
    '/root/permissions': ['BUG-03', 'PERF-02'],
    '/root/migration': ['BUG-02'],
    '/root/billing_recovery': ['BUG-04/05复杂规划（已终止实施路由）'],
    '/root/billing_implementation': ['BUG-04', 'BUG-05'],
    '/root/local_ops': ['PERF-01', 'OPS-04', 'DEBT-03', 'TEST-04', 'OPS-02', 'OPS-03', 'OPS-05'],
    '/root/operator_ux': ['UX-01', 'UX-02', 'UX-03', 'UX-04', 'DEBT-02', 'TEST-02'],
    '/root/admin_boundaries': ['DEBT-01'],
    '/root/operator_completion': ['UX-01', 'UX-02', 'UX-03', 'UX-04', 'DEBT-02', 'TEST-02（Terra未闭环后接手）'],
    '/root/recovery_gap_plan': ['BUG-04追加缺口规划/独立只读复核'],
    '/root/recovery_foreground_fence': ['BUG-04前台幂等fence/隔离保护实施'],
    '/root/billing_queue_fallback': ['BUG-04普通计费队列兜底实施'],
    '/root/authoritative_balance_hit': ['BUG-04主库余额命中分支实施'],
    '/root/sqlite_wal_fix': ['BUG-04 SQLite WAL引擎最小修复版本升级'],
    '/root/predeploy_plan_review': ['部署前迁移/回滚/候选来源独立只读复核'],
    '/root/predeploy_frontend_clean': ['部署前干净frozen安装/TS6307复验/前端构建'],
    '/root/predeploy_race_checks': ['部署前Windows race验证/分类/Gin测试初始化修正'],
    '/root/race_monitor_fixture': ['部署前monitor并发测试fixture同步修复'],
    '/root/remaining_gap_audit': ['剩余高价值问题独立只读复核'],
    '/root/v025_conflict_audit': ['官方 v0.2.5 合并冲突与语义风险只读审计'],
    '/root/v025_backend_conflicts': ['官方 v0.2.5 后端冲突融合与定向验证'],
    '/root/v025_frontend_conflicts': ['官方 v0.2.5 前端冲突融合与定向验证'],
}


def read_rollout(path):
    meta = None
    models = []
    counts = []
    errors = 0
    for line in path.open(encoding='utf-8'):
        try:
            event = json.loads(line)
        except (ValueError, OSError):
            continue
        kind, value = event.get('type'), event.get('payload', {})
        if kind == 'session_meta':
            meta = value
        elif kind == 'turn_context':
            models.append({'timestamp': event.get('timestamp'), 'model': value.get('model'), 'effort': value.get('effort')})
        elif kind == 'event_msg' and value.get('type') == 'token_count' and value.get('info'):
            counts.append({'timestamp': event['timestamp'], 'usage': {k: value['info']['total_token_usage'].get(k) for k in FIELDS},
                           'weekly': (value.get('rate_limits') or {}).get('primary')})
        elif kind == 'event_msg' and value.get('type') == 'error':
            errors += 1
    return meta, models, counts, errors


def usage_since(counts, start=None):
    """Accumulate deltas across counter resets; do not sum cumulative events."""
    totals = dict.fromkeys(FIELDS, 0)
    previous = dict.fromkeys(FIELDS, 0)
    resets = []
    for count in counts:
        current = count['usage']
        restarted = current.get('total_tokens') is not None and previous.get('total_tokens') is not None and current['total_tokens'] < previous['total_tokens']
        included = start is None or count['timestamp'] >= start
        if included:
            if restarted:
                resets.append(count['timestamp'])
            for key in FIELDS:
                value = current.get(key)
                before = 0 if restarted else previous.get(key)
                if value is None or before is None:
                    totals[key] = None
                elif totals[key] is not None:
                    totals[key] += max(0, value - before)
        previous = current
    return totals, resets


def main():
    previous = REPORT.read_text(encoding='utf-8') if REPORT.exists() else ''
    # Preserve a timestamped previous acceptance estimate when refreshing actual
    # measurements during a resumed run. It is not the current phase's estimate.
    previous_marker = '\n## 最终逐项反事实基准（仅估算）'
    previous_estimate = previous.split(previous_marker, 1)[1] if previous_marker in previous else ''
    for comparison_marker in (
        '\n## 最终比较及下一轮规则',
        '\n## 当前最终比较及下一轮规则',
    ):
        if comparison_marker in previous_estimate:
            previous_estimate = previous_estimate.split(comparison_marker, 1)[0]
    previous_estimate = previous_estimate.rstrip()
    tid = os.environ.get('CODEX_THREAD_ID')
    if not tid:
        raise SystemExit('CODEX_THREAD_ID required; refusing to inspect unrelated task content')
    home = Path(os.environ['USERPROFILE']) / '.codex' / 'sessions' / '2026' / '09'
    agents = []
    for day in ['15', '16', '17', '18']:
        directory = home / day
        if not directory.exists():
            continue
        for path in directory.glob('*.jsonl'):
            with path.open(encoding='utf-8') as stream:
                try:
                    meta = json.loads(next(stream)).get('payload', {})
                except (ValueError, StopIteration):
                    continue
            spawn = ((meta.get('source') or {}).get('subagent', {}).get('thread_spawn', {})
                     if isinstance(meta.get('source'), dict) else {})
            root = meta.get('id') == tid
            if not root and spawn.get('parent_thread_id') != tid:
                continue
            _, models, counts, errors = read_rollout(path)
            if not counts:
                continue
            before = [c for c in counts if c['timestamp'] < START] if root else []
            last = counts[-1]
            usage, resets = usage_since(counts, START if root else None)
            name = '/root' if root else spawn.get('agent_path', 'unknown')
            agents.append({'agent': name, 'tasks': ASSIGNMENTS.get(name, ['见派发记录']), 'models': models,
                           'usage': usage, 'last_sample': last['timestamp'], 'weekly_last': last['weekly'],
                           'weekly_baseline': before[-1]['weekly'] if before else None,
                           'stream_error_events': errors, 'counter_resets': resets})
    snapshot = {'sampled_at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'start': START, 'agents': agents}
    OUT.mkdir(exist_ok=True)
    (OUT / 'usage-latest.json').write_text(json.dumps(snapshot, ensure_ascii=False, indent=2), encoding='utf-8')
    with (OUT / 'usage-snapshots.jsonl').open('a', encoding='utf-8') as stream:
        stream.write(json.dumps(snapshot, ensure_ascii=False) + '\n')
    totals = {k: sum(a['usage'][k] or 0 for a in agents) for k in FIELDS}
    weekly_path = OUT / 'weekly-samples.jsonl'
    weekly = [json.loads(line) for line in weekly_path.read_text(encoding='utf-8-sig').splitlines() if line.strip()] if weekly_path.exists() else []
    weekly_latest = weekly[-1].get('usedPercent', weekly[-1].get('used_percent', '未读到')) if weekly else '未读到'
    lines = [
        '# 模型路由与额度效率审计', '',
        f'更新时间（UTC）：{snapshot["sampled_at"]}。持续抽样文件：`.local/autonomous-backlog/usage-snapshots.jsonl`。', '',
        '## 统计口径与可读性', '',
        '- 实测来源：当前主线程及其直接子agent的本地rollout `token_count.total_token_usage`，模型/effort来自turn_context。仅提取数字与模型元数据，不复制消息、工具输出、认证信息或账户ID。',
        f'- 主代理基线：自主执行turn_context `{START}` 前最后一条累计usage；子agent从零计。此前扫描消耗排除。事件可能延迟入盘，当前数字是已记录下界。',
        '- cached input 是 input 的子集；reasoning output 是 output 的细分，不另加到total，避免双计。累计事件按相邻增量计算，重复事件不增加；total下降识别计数重置，新段从零累加，主线程仅纳入自主起点后增量。',
        '- 继续执行时发现主线程累计计数在自主开始及后续恢复时重置；已改为分段累计。先前81,297,920截止快照采用末值减旧基线，因此少计首次重置前的1,035,041；旧快照保留为历史证据，上方最新分段数字为准，不将计数下降当负消耗。',
        '- 同一agent并行/交错处理多个条目时只能实测该任务组，逐条拆分不可直接测量；不得把估计分摊当实测。主代理协调、审计和终验开销包含在实际总消耗。',
        f'- 账户周窗口10080分钟；usage tool首个实时读数4%，最新数字快照{weekly_latest}% used，自主执行前日志2%。这是账户共享且量化的快照，差额不能精确归因本任务或模型；存在其他线程、延迟及取整误差。',
        '- 当前模型catalog不提供周额度的每token/缓存/模型换算权重。API价格、API TPM和Codex订阅周额度不是同一口径；不把API缓存折扣或API价格冒充周额度公式。',
        '- 官方字段背景：[Prompt caching](https://developers.openai.com/api/docs/guides/prompt-caching)。该资料支持缓存是输入细分，不能据此推算当前账户周额度。', '',
        '## 实测任务组', '',
        '| Agent / 条目 | 实际模型/effort | Input | Cached input | Output（含reasoning） | Reasoning细分 | 逐项周额度 |',
        '| --- | --- | ---: | ---: | ---: | ---: | --- |',
    ]
    for agent in sorted(agents, key=lambda x: x['agent']):
        model = agent['models'][-1] if agent['models'] else {}
        u = agent['usage']
        lines.append(f'| {agent["agent"]} / {", ".join(agent["tasks"])} | {model.get("model", "未读到")} / {model.get("effort", "未读到")} | {u["input_tokens"]:,} | {u["cached_input_tokens"]:,} | {u["output_tokens"]:,} | {u["reasoning_output_tokens"]:,} | 无法独立读取 |')
    model_totals = {}
    for agent in agents:
        model = agent['models'][-1].get('model', '未知') if agent['models'] else '未知'
        aggregate = model_totals.setdefault(model, {'groups': 0, **dict.fromkeys(FIELDS, 0)})
        aggregate['groups'] += 1
        for key in FIELDS:
            aggregate[key] += agent['usage'][key] or 0
    lines += ['', '## 各模型实测汇总（任务组包含规划、实施及复核）', '',
              '| 模型 | 任务组 | Input | Cached input | Output | Reasoning细分 | 实际周额度 |',
              '| --- | ---: | ---: | ---: | ---: | ---: | --- |']
    for model, aggregate in sorted(model_totals.items()):
        lines.append(f'| {model} | {aggregate["groups"]} | {aggregate["input_tokens"]:,} | {aggregate["cached_input_tokens"]:,} | {aggregate["output_tokens"]:,} | {aggregate["reasoning_output_tokens"]:,} | 无法按模型读取 |')
    lines += ['', '## 追加已闭环子任务的 Sol High 反事实（估算）', '',
              '以下采用观测轨迹重放、token倍率1及相同缓存条件；不是预测 Sol 的真实推理量，也不是周额度。主代理复核与共享规划未逐项分摊，包含在全程基准；BUG-04整体仍部分完成。', '',
              '| 已验证子任务 | 实际任务组 total tokens | Sol High 重放 tokens（估算） | 周额度节省 |',
              '| --- | ---: | ---: | --- |']
    for a in sorted(agents, key=lambda x: x['agent']):
        if a['agent'] in {'/root/recovery_foreground_fence', '/root/billing_queue_fallback', '/root/authoritative_balance_hit', '/root/predeploy_frontend_clean', '/root/race_monitor_fixture', '/root/predeploy_plan_review', '/root/predeploy_race_checks'}:
            total = a['usage']['total_tokens']
            lines.append(f'| {a["agent"]} | {total:,} | 约 {total:,}（倍率1假设） | 权重未知，无法量化 |')
    lines += ['', f'已入盘实际总量：input **{totals["input_tokens"]:,}**，cached input **{totals["cached_input_tokens"]:,}**，output **{totals["output_tokens"]:,}**，reasoning细分 **{totals["reasoning_output_tokens"]:,}**；total **{totals["total_tokens"]:,}**。', '',
              '## Sol High 反事实（估算，不是实测）', '',
              '- 每个条目归属上表；完成时在 routing-events.jsonl 记录完成/失败/升级边界。尚未完成的任务不生成最终反事实金额。',
              '- 同上下文、同缓存、同结果的固定工作轨迹重放基准：反事实 input/cached/output 先取对应实测任务组值，模型全部替换为 Sol/high；这是等工作量假设，不能证明换模型仍需相同推理量。Sol/high自身任务组的反事实token重放倍率为1；root medium→high的真实推理变化无法测量。',
              '- 反事实周额度 = Σ[(I-C)×w_Sol_input + C×w_Sol_cache + O×w_Sol_output]；实际额度采用各模型各自权重。权重当前未知，保留公式，不伪造精确周额度、总额或节省百分比。',
              '- 下一轮有不同模型的可比闭环或可读计费权重时再校准；不为本轮审计额外重复整个开发任务。', '',
              '## 失败、重试与模型升级（已发生）', '',
              '- 环境失败：旧Git index.lock（未删除/未改index）、WSL E_ACCESSDENIED、默认D:/GoCache拒绝访问、新Go缓存首次建桶限制。改用文件checkpoint、Windows portable PG及预建独立cache；不是低模型能力失败。',
              '- root两次Python修改脚本因默认GBK解码失败，其中首次快照因git write-tree旧锁失败；均明确失败，修正为UTF8/只读index SHA256后校验。',
              '- scheduling首个日志写权限失败、第一次有效测试复验已退出1；随后默认缓存环境失败，独立cache重跑。没有把空退出码或无测试运行认作通过。',
              '- Luna提前结束但未完成最终验证，主代理退回；锁文件及离线计划安全缺口由Sol/medium接管、Terra补强，属于质量返工/移交，有额外开销。环境故障另记。Terra/operator连续未闭环后，异步权限隔离与真实浏览器问题升级Sol/high接手，已发生低模型→高模型重做，具体额外额度无法独立读取。Astra规划移交属于用户路由纠正，不是能力失败。', '',
              '- 追加修复：规划给错 unit 测试路径、Luna fixture 的双返回值误用，均在同模型更正；有效行为 red/green 另存。主代理真实 PG fixture 首次 nil Ent client 导致 panic，修复后真实集成通过；不得将该 panic 算旧实现行为 red。',
              '- SQLite 最小升级前发现写缓存被拒绝，以及默认 GOPATH sumdb 落在只读外部目录；修复本地缓存路径并保留校验，不触发模型升级。完整 backend 第二轮被主代理主动中断以避免升级中混合依赖，须重新完整验证。合成 PG 首次漏显式监听参数已停止并仅在 loopback25433 重开；这些是环境/协调失败。', '',
              '- Docker/Linux终验：c07完整race发现真实Account缓存竞争，由root Sol/medium修复；没有低模型失败或模型升级。随后一次完整race只因race插桩下既有分配阈值过低而失败，校准阈值并定向20轮、完整race复验通过。standard首次因本地PG夹具启动顺序失败；Docker日志GBK解码、迁移列名和PID1 UID验证各修正一次，均属主代理命令/环境重试。', '',
              '## 路由计数与阶段性建议', '',
              '- 每个模型承接组与条目以实测任务表及routing-events为准。初批Sol/high三组承接5条；后续Astra规划移交Sol，Luna机械4条、Terra前端6条。完成数以backlog闭环为准，不把派发数当完成数。',
              '- 已发生的主要重复开销来自环境适配与独立cache首次编译；下一批先共享确认可读的运行环境并限并发编译，避免升级模型解决环境问题。',
              '- 委派前核对真实测试文件路径、build tag 与所用 API 返回签名，并确认 GOPATH/GOMODCACHE/GOCACHE 全部本地可写；给出可观察退出码，减少上下文不足造成的返工。',
              '- 不以少量样本推断所有模型成功率和节省额。新正式规则：Astra/xhigh仅最复杂规划与关键决策，完成规划后移交满足风险的最低成本模型实施；环境错误不触发模型升级。', '',
              '- 本轮实证补充：先完成一次干净frozen构建，不能用增量TS缓存成功替代composite项目验收；命令用结构化argv并核对-p参数。共享源码批量编辑/helper创建期间不启动包编译，先确认稳定。桌面入口须读技能并测试实际应用权限，不能由浏览器CUA限制推断全部Windows能力不可用。Docker启动权限、Git最小环境unsafe ownership和清单来源标签错误均是环境/协调/元数据问题，不触发模型升级。',
              '- 轨迹重放反事实保留观测失败与返工，仅用于固定工作量比较；全程Sol High可能避免部分低模型失误，也可能增加推理量，未经同任务对照无法量化。不能把重放基准当成最优Sol High实际消耗。', '',
              '## 实时事件记录', '',
              '独立追加事件见 `.local/autonomous-backlog/routing-events.jsonl`；本报告由 tools/model-routing-audit.py 刷新，持续保留历史数字快照。']
    if previous_estimate:
        lines += ['', '## 前阶段验收估算快照', '',
                  '以下逐项均摊与计数仅对应前阶段截止抽样；本次继续修复的实测用量以上方最新任务组为准。',
                  '## 最终逐项反事实基准（仅估算）' + previous_estimate]
    group_counts = ', '.join(f'{model} {data["groups"]}组' for model, data in sorted(model_totals.items()))
    lines += ['', '## 当前最终比较及下一轮规则', '',
              f'- **实测**动态路由累计：Input {totals["input_tokens"]:,}、Cached input {totals["cached_input_tokens"]:,}、Output {totals["output_tokens"]:,}（其中reasoning {totals["reasoning_output_tokens"]:,}）、Total {totals["total_tokens"]:,}。任务组：{group_counts}。',
              f'- **实测**账户共享周窗口最新为{weekly_latest}% used；自主前日志2%、首次实时工具读数4%。该窗口包含其他线程且按整数取整，不能把它与历史快照的差值精确归因本任务，更不能拆到模型。',
              f'- **估算**全程Sol High按相同轨迹、上下文、缓存和结果重放：Input {totals["input_tokens"]:,}、Cached input {totals["cached_input_tokens"]:,}、Output {totals["output_tokens"]:,}、Total {totals["total_tokens"]:,}。这是token倍率1基准，不是实际Sol High对照实验。',
              '- 动态路由与全程Sol High的周额度权重均不可读，因此预计节省百分比为**不可识别（N/A）**；不能把token相同误写为0%额度节省。',
              '- 按低成本模型承担的实测token规模，Terra的DEBT-01与operator任务组、predeploy race组，以及Luna的主库余额分支是潜在节省最大的组；缺少模型额度权重，不能给出净节省排名。operator组及早期Luna本地运维发生返工，不能称为确定净节省。',
              '- 明确的低模型→高模型重做：Luna本地运维过早结束后由主代理Sol/Terra补齐；Terra operator未闭环后由Sol/high接手。独立失败段token不可分离。Astra仅做复杂规划并移交，属于按规则路由，不算失败升级。Docker/Linux终验由Sol/medium完成，没有Astra或新一轮模型升级。',
              '- 下一轮先由主owner验证工具链、缓存、build tag和合成本地DB，再派发实现；机械任务用Luna，常规边界用Terra，账务、权限、并发与关键审查用Sol。第一次契约失败同层返工，重复未闭环或具体复杂性证据才升级；环境故障不升级。Astra/xhigh只做最复杂规划，方案拆解后交给最低充分成本模型实施。',
              '- 主代理协调与终验占用最多重复缓存输入。下一轮应缩短fork上下文、一次明确文件所有权和验收命令、避免共享源码处于中间态时启动测试，并只在变更或失败证据要求时扩大测试。']
    REPORT.write_text('\n'.join(lines) + '\n', encoding='utf-8')
    print('Audit metadata sampled:', len(agents), 'task groups; total tokens:', totals['total_tokens'])


if __name__ == '__main__':
    main()
