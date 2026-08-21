<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import { ApiError } from '../api/client'
import * as walletApi from '../api/wallet'
import type { RechargeOrder, WalletLedger, WalletSummary } from '../api/wallet'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()

const loading = ref(false)
const busy = ref(false)
const summary = ref<WalletSummary | null>(null)
const packages = ref<walletApi.RechargePackage[]>([])
const ledgers = ref<WalletLedger[]>([])
const customYuan = ref('')
const payImage = ref('')
const payingNo = ref('')
const payYuanAmount = ref(0)
const orderHint = reactive({
  text: '',
  tone: '' as '' | 'ok' | 'bad',
})

let pollTimer: number | undefined
let pollSeq = 0
const paying = computed(() => payImage.value !== '')
const POLL_INTERVAL_MS = 2000
const POLL_MAX_TRIES = 150

function formatTime(raw?: string) {
  if (!raw) return ''
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return raw
  return d.toLocaleString()
}

async function loadWallet() {
  if (!auth.isLoggedIn) return
  loading.value = true
  try {
    const [sumRes, ledgerRes] = await Promise.all([walletApi.summary(), walletApi.listLedger()])
    summary.value = sumRes.summary
    packages.value = sumRes.packages ?? []
    ledgers.value = ledgerRes.ledgers ?? []
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

function packageCoins(pkg: walletApi.RechargePackage) {
  return pkg.yuan * 10 + pkg.bonus
}

async function payYuan(yuan: number) {
  if (!auth.isLoggedIn) {
    toast.error('请先登录')
    await router.push('/account')
    return
  }
  if (busy.value) return
  if (!Number.isInteger(yuan) || yuan < 1) {
    toast.error('请输入不少于 1 的整元金额')
    return
  }
  busy.value = true
  try {
    const res = await walletApi.createRecharge(yuan)
    if (!res.qr_image) {
      throw new Error('未拿到支付码')
    }
    payImage.value = res.qr_image
    payingNo.value = res.order.out_trade_no
    payYuanAmount.value = res.order.yuan
    void pollOrder(res.order.out_trade_no)
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e))
  } finally {
    busy.value = false
  }
}

function stopPoll() {
  pollSeq += 1
  if (pollTimer !== undefined) {
    window.clearTimeout(pollTimer)
    pollTimer = undefined
  }
}

function closePay() {
  payImage.value = ''
  payingNo.value = ''
  payYuanAmount.value = 0
  stopPoll()
}

async function payCustom() {
  const yuan = Number(customYuan.value)
  await payYuan(yuan)
}

async function pollOrder(outTradeNo: string) {
  const seq = ++pollSeq
  orderHint.text = '支付处理中…'
  orderHint.tone = ''
  let tries = 0
  const tick = async () => {
    if (seq !== pollSeq) return
    tries += 1
    try {
      const res = await walletApi.queryRecharge(outTradeNo)
      if (seq !== pollSeq) return
      const order: RechargeOrder = res.order
      if (order.status === 'paid') {
        orderHint.text = `充值成功，到账 ${order.coins + order.bonus} 积分`
        orderHint.tone = 'ok'
        closePay()
        await loadWallet()
        return
      }
      if (order.status === 'closed') {
        orderHint.text = '订单已关闭，请重新充值'
        orderHint.tone = 'bad'
        closePay()
        return
      }
    } catch (e) {
      if (seq !== pollSeq) return
      orderHint.text = e instanceof ApiError ? e.message : '查询订单失败'
      orderHint.tone = 'bad'
      return
    }
    if (seq !== pollSeq) return
    // 每 2 秒查一次，最多约 5 分钟；入账只认服务端查单，不跳沙箱收银台。
    if (tries >= POLL_MAX_TRIES) {
      orderHint.text = '仍在处理，稍后刷新钱包即可'
      return
    }
    pollTimer = window.setTimeout(() => {
      void tick()
    }, POLL_INTERVAL_MS)
  }
  await tick()
}

onMounted(() => {
  if (!auth.isLoggedIn) {
    void router.push('/account')
    return
  }
  void loadWallet()
})

onUnmounted(() => {
  stopPoll()
})
</script>

<template>
  <AppShell>
    <div class="card">
      <div class="row" style="justify-content: space-between">
        <p class="title" style="margin: 0">钱包</p>
        <div class="row" style="gap: 8px">
          <button class="ghost" type="button" @click="router.push('/checkin')">签到</button>
          <button class="ghost" type="button" @click="router.push('/lottery')">抽奖</button>
          <button class="ghost" type="button" @click="router.push('/account')">返回账号</button>
        </div>
      </div>
      <div v-if="orderHint.text" class="hint" :class="orderHint.tone" style="margin-top: 12px">{{ orderHint.text }}</div>
      <div v-if="loading && !summary" class="subtle" style="margin-top: 12px">加载中…</div>
      <template v-else-if="summary">
        <div class="balance">{{ summary.available_coins }} <span class="unit">积分</span></div>
        <p v-if="summary.expiring_soon_coins > 0" class="subtle warn">
          3 天内将过期 {{ summary.expiring_soon_coins }} 积分
          <template v-if="summary.next_expire_at">
            ，最近一笔 {{ summary.next_expire_coins }} 积分将于 {{ formatTime(summary.next_expire_at) }} 过期
          </template>
        </p>
        <p v-else class="subtle">暂无即将过期的积分</p>
      </template>
    </div>

    <div class="card">
      <p class="title">充值</p>
      <p class="subtle">按元支付，1 元 = 10 积分。档位赠送一并进入充值余额，不过期。请用沙箱支付宝扫码，不要用真支付宝。</p>
      <div class="pkg-grid">
        <button
          v-for="pkg in packages"
          :key="pkg.yuan"
          class="pkg"
          type="button"
          :disabled="busy || paying"
          @click="payYuan(pkg.yuan)"
        >
          <div class="pkg-yuan">{{ pkg.yuan }} 元</div>
          <div class="pkg-coin">到账 {{ packageCoins(pkg) }} 积分</div>
          <div v-if="pkg.bonus > 0" class="pkg-bonus">含赠送 {{ pkg.bonus }}</div>
        </button>
      </div>
      <div class="row" style="margin-top: 12px">
        <input v-model.trim="customYuan" type="number" min="1" step="1" placeholder="自定义整元，无赠送" />
        <button class="primary" type="button" :disabled="busy || paying" @click="payCustom">充值</button>
      </div>
    </div>

    <div class="card">
      <p class="title">流水</p>
      <div v-if="ledgers.length === 0" class="subtle">暂无流水</div>
      <div v-for="item in ledgers" :key="item.id" class="ledger">
        <div>
          <div>{{ walletApi.ledgerLabel(item.biz_type) }}</div>
          <div class="subtle">{{ formatTime(item.created_at) }}</div>
        </div>
        <div class="ledger-amt" :class="{ plus: item.amount > 0, minus: item.amount < 0 }">
          {{ item.amount > 0 ? '+' : '' }}{{ item.amount }}
        </div>
      </div>
    </div>

    <Teleport to="body">
      <div v-if="payImage" class="pay-mask" @click.self="closePay">
        <div class="pay-dialog" role="dialog" aria-modal="true" aria-labelledby="pay-title">
          <p id="pay-title" class="title" style="margin: 0">扫码充值</p>
          <img class="pay-qr" :src="payImage" alt="充值二维码" />
          <p class="subtle">请用沙箱支付宝扫码支付 {{ payYuanAmount }} 元，不要用真支付宝。</p>
          <p class="subtle">扫码后请保持此窗口，到账由服务器确认。</p>
          <button class="ghost" type="button" @click="closePay">关闭</button>
        </div>
      </div>
    </Teleport>
  </AppShell>
</template>

<style scoped>
.ghost {
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(0, 0, 0, 0.18);
  color: rgba(255, 255, 255, 0.86);
  border-radius: 12px;
  padding: 10px 12px;
  cursor: pointer;
}

.balance {
  margin-top: 12px;
  font-size: 36px;
  font-weight: 800;
}

.unit {
  font-size: 16px;
  font-weight: 500;
  color: rgba(255, 255, 255, 0.64);
}

.warn {
  color: #fbbf24;
}

.hint {
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.06);
}

.hint.ok {
  color: #86efac;
}

.hint.bad {
  color: #fda4af;
}

.pkg-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: 12px;
}

.pkg {
  text-align: left;
  padding: 14px;
}

.pkg-yuan {
  font-weight: 700;
}

.pkg-coin,
.pkg-bonus {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.64);
}

.pkg-bonus {
  color: #fbbf24;
}

.pay-mask {
  position: fixed;
  inset: 0;
  z-index: 40;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: rgba(0, 0, 0, 0.62);
}

.pay-dialog {
  width: min(360px, 100%);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 20px;
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: #16161c;
}

.pay-qr {
  width: 220px;
  height: 220px;
  border-radius: 12px;
  background: #fff;
  padding: 8px;
}

.ledger {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 0;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.ledger-amt.plus {
  color: #86efac;
}

.ledger-amt.minus {
  color: #fda4af;
}
</style>
