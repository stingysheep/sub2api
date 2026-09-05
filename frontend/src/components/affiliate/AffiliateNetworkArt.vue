<template>
  <div class="affiliate-network-art" :class="{ compact }" aria-hidden="true">
    <span class="ring outer"></span><span class="ring inner"></span>
    <span v-for="side in sides" :key="side" class="line" :class="side"></span>
    <span class="node top"><Icon name="user" size="sm" /></span>
    <span class="node right"><Icon name="user" size="sm" /></span>
    <span class="node bottom"><Icon name="dollar" size="sm" /></span>
    <span class="node left"><Icon name="user" size="sm" /></span>
    <span class="core"><Icon name="link" :size="compact ? 'md' : 'lg'" /></span>
  </div>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
withDefaults(defineProps<{ compact?: boolean }>(), { compact: false })
const sides = ['top', 'right', 'bottom', 'left']
</script>

<style scoped>
.affiliate-network-art{position:relative;width:300px;height:220px;color:rgb(5 150 105)}
.affiliate-network-art.compact{transform:scale(.78);transform-origin:center}
.ring,.line,.node,.core{position:absolute}.ring{left:50%;top:50%;transform:translate(-50%,-50%);border:1px dashed rgb(167 243 208);border-radius:999px}.ring.outer{width:188px;height:188px}.ring.inner{width:116px;height:116px;border-color:rgb(209 250 229)}
.line{left:50%;top:50%;width:92px;height:1px;transform-origin:left center;background:linear-gradient(90deg,rgb(167 243 208),transparent)}.line.top{transform:rotate(-90deg)}.line.right{transform:rotate(0)}.line.bottom{transform:rotate(90deg)}.line.left{transform:rotate(180deg)}
.node,.core{display:flex;align-items:center;justify-content:center;background:rgb(255 255 255 / .94);box-shadow:0 8px 24px rgb(15 118 110 / .1);will-change:transform,opacity}.node{width:34px;height:34px;border:1px solid rgb(110 231 183);border-radius:999px;animation:float 7.2s cubic-bezier(.45,.05,.55,.95) infinite}.node.top{left:133px;top:5px;animation-delay:-.4s}.node.right{right:12px;top:94px;animation-delay:-1.5s}.node.bottom{left:133px;bottom:5px;color:rgb(217 119 6);border-color:rgb(253 230 138);animation-delay:-2.6s}.node.left{left:12px;top:94px;animation-delay:-3.1s}
.core{left:50%;top:50%;width:70px;height:70px;transform:translate(-50%,-50%);border:1px solid rgb(110 231 183);border-radius:16px;animation:pulse 6.6s cubic-bezier(.45,.05,.55,.95) infinite}
@keyframes float{0%,100%{transform:translate3d(0,0,0);opacity:.92}50%{transform:translate3d(0,-6px,0);opacity:1}}@keyframes pulse{0%,100%{transform:translate(-50%,-50%) scale(1);opacity:.94}50%{transform:translate(-50%,-50%) scale(1.04);opacity:1}}
@media(prefers-reduced-motion:reduce){.node,.core{animation:none}}
</style>
