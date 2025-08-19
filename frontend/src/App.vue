<template>
  <n-config-provider :theme="darkTheme">
    <n-message-provider>
      <n-dialog-provider>
        <n-global-style />
        <router-view />
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { NConfigProvider, NGlobalStyle, darkTheme, NMessageProvider, NDialogProvider } from 'naive-ui'
</script>
