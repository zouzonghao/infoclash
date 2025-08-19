<template>
  <div style="padding: 20px">
    <n-card style="margin-bottom: 16px">
      <n-form @submit.prevent="handleSearch">
        <n-grid :cols="24" :x-gap="24">
          <n-form-item-gi :span="6" label="Host">
            <n-input
              v-model:value="filters.host"
              placeholder="输入 Host 搜索"
              @input="filters.host = filters.host.replace(/\s/g, '')"
            clearable />
          </n-form-item-gi>
          <n-form-item-gi :span="4" label="Source IP">
            <n-input
                v-model:value="filters.sourceIP"
                placeholder="输入 Source IP 搜索"
                @input="filters.sourceIP = filters.sourceIP.replace(/\s/g, '')"
                clearable />
          </n-form-item-gi>
          <n-form-item-gi :span="4" label="Chain">
            <n-select
              v-model:value="filters.chain"
              placeholder="选择 Chain"
              :options="chainOptions"
              clearable
            />
          </n-form-item-gi>
          <n-form-item-gi :span="4" label="预设范围">
            <n-select
              v-model:value="timePreset"
              placeholder="选择预设范围"
              :options="timePresetOptions"
              clearable
            />
          </n-form-item-gi>
          <n-form-item-gi :span="6" label="时间范围">
            <n-date-picker v-model:value="timeRange" type="datetimerange" clearable style="width: 100%" />
          </n-form-item-gi>
        </n-grid>
        <n-row :gutter="[0, 24]">
          <n-col :span="24">
            <div style="display: flex; justify-content: space-between; align-items: center; width: 100%;">
              <div>
               <n-button @click="showMergeModal = true" style="margin-right: 8px;"> 数据库精简 </n-button>
               <n-button @click="showReplaceHostModal = true"> 域名替换 </n-button>
              </div>
              <n-pagination
                v-model:page="page"
                v-model:page-size="pageSize"
                :item-count="data?.total || 0"
                :page-sizes="[10, 20, 50, 100]"
                show-size-picker
                size="small"
              />
              <div>
                <n-button @click="handleReset">重置</n-button>
                <n-button attr-type="submit" style="margin-left: 8px"> 搜索 </n-button>
              </div>
            </div>
          </n-col>
        </n-row>
      </n-form>
    </n-card>

    <n-card v-if="filters.host && data?.data?.length > 0" style="margin-bottom: 16px">
      <Line :data="chartData" :options="chartOptions" />
    </n-card>

    <n-data-table
      :columns="columns"
      :data="data?.data || []"
      :bordered="false"
      :scroll-x="1200"
      :loading="isLoading"
      :remote="true"
      @update:sorter="handleSorterChange"
    />

    <n-modal
      v-model:show="showMergeModal"
      preset="dialog"
      title="数据库精简"
      positive-text="确认合并"
      negative-text="取消"
      :loading="isMerging"
      @positive-click="handleMerge"
    >
      <n-space vertical>
        <n-form-item label="时间范围" required>
          <n-date-picker v-model:value="mergeTimeRange" type="datetimerange" style="width: 100%" />
        </n-form-item>
        <n-form-item label="合并间隔" required>
          <n-select v-model:value="mergeInterval" :options="mergeIntervalOptions" />
        </n-form-item>
      </n-space>
    </n-modal>

    <n-modal
      v-model:show="showReplaceHostModal"
      preset="dialog"
      title="域名替换"
      positive-text="确认替换"
      negative-text="取消"
      :loading="isReplacingHost"
      @positive-click="handleReplaceHost"
    >
      <n-form-item label="域名后缀" required>
        <n-input v-model:value="replaceDomainSuffix" placeholder="输入要替换的域名后缀" />
      </n-form-item>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { h, ref, computed, reactive, watch } from 'vue'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import {
  NDataTable,
  NCard,
  NForm,
  NGrid,
  NFormItemGi,
  NInput,
  NSelect,
  NDatePicker,
  NButton,
  NRow,
  NCol,
  NModal,
  useMessage,
  useDialog,
  NDialog,
  NIcon,
  NSpace,
  NFormItem,
  NPagination
} from 'naive-ui'
import axios from 'axios'
import { useQuery, useMutation } from '@tanstack/vue-query'
import dayjs from 'dayjs'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler)

// 响应式状态
const filters = ref({
  host: '',
  sourceIP: '',
  chain: '',
  startDate: null as number | null,
  endDate: null as number | null
})
const timeRange = ref<[number, number] | null>(null)
const timePreset = ref<string | null>(null)
const showMergeModal = ref(false)
const showReplaceHostModal = ref(false)
const replaceDomainSuffix = ref('')
const message = useMessage()
const dialog = useDialog()
const mergeTimeRange = ref<[number, number] | null>(null)
const mergeInterval = ref<number>(10)
const mergeIntervalOptions = [
 { label: '10分钟', value: 10 },
 { label: '30分钟', value: 30 },
 { label: '1小时', value: 60 },
 { label: '3小时', value: 180 },
 { label: '6小时', value: 360 },
 { label: '12小时', value: 720 },
 { label: '1天', value: 1440 }
]

const timePresetOptions = [
  { label: '过去5分钟', value: '5m' },
  { label: '过去10分钟', value: '10m' },
  { label: '过去20分钟', value: '20m' },
  { label: '过去30分钟', value: '30m' },
  { label: '过去1小时', value: '1h' },
  { label: '过去6小时', value: '6h' },
  { label: '过去12小时', value: '12h' },
  { label: '过去1天', value: '1d' },
  { label: '过去3天', value: '3d' },
  { label: '过去1周', value: '1w' }
]

const sorter = ref({
  key: 'start',
  order: 'descend'
})
const page = ref(1)
const pageSize = ref(10)

const formatBytes = (bytes: number) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const fetchConnections = async ({ queryKey }: { queryKey: any }) => {
  const [_key, { page, pageSize, sorter, filters }] = queryKey
  const params = new URLSearchParams({
    page: page.toString(),
    pageSize: pageSize.toString(),
    sortBy: sorter.key || 'start',
    sortOrder: sorter.order === 'descend' ? 'desc' : 'asc'
  })

  if (filters.host) params.append('host', filters.host)
  if (filters.sourceIP) params.append('sourceIP', filters.sourceIP)
  if (filters.chain) params.append('chain', filters.chain)
  if (filters.startDate) params.append('startDate', filters.startDate.toString())
  if (filters.endDate) params.append('endDate', filters.endDate.toString())
  console.log('Request URL:', `/api/connections?${params.toString()}`)
  const response = await axios.get('/api/connections', { params })
  console.log('Response data:', response.data)
  return response.data
}

const queryKey = computed(() => [
  'connections',
  {
    page: page.value,
    pageSize: pageSize.value,
    sorter: sorter.value,
    filters: filters.value
  }
])

const { data, isLoading, refetch } = useQuery({
  queryKey,
  queryFn: fetchConnections
})


// 获取 Chain 选项
const { data: chainOptions } = useQuery({
  queryKey: ['chains'],
  queryFn: async () => {
    const response = await axios.get('/api/chains')
    return response.data.map((chain: string) => ({ label: chain, value: chain }))
  }
})

watch(timePreset, (newVal) => {
  if (newVal) {
    setTimeRangePreset(newVal)
  } else {
    timeRange.value = null
    timePreset.value = null
  }
})

watch(timeRange, (newVal) => {
  if (newVal) {
    filters.value.startDate = dayjs(newVal[0]).unix()
    filters.value.endDate = dayjs(newVal[1]).unix()
  } else {
    filters.value.startDate = null
    filters.value.endDate = null
  }
})

watch(pageSize, () => {
  page.value = 1
})

const setTimeRangePreset = (preset: string) => {
  const now = dayjs()
  let start = now
  const unit = preset.slice(-1)
  const value = parseInt(preset.slice(0, -1))

  switch (unit) {
    case 'm':
      start = now.subtract(value, 'minute')
      break
    case 'h':
      start = now.subtract(value, 'hour')
      break
    case 'd':
      start = now.subtract(value, 'day')
      break
    case 'w':
      start = now.subtract(value, 'week')
      break
  }

  timeRange.value = [start.valueOf(), now.valueOf()]
}

const handleSorterChange = (sorterInfo: any) => {
  if (!sorterInfo || !sorterInfo.columnKey) {
    sorter.value = { key: 'start', order: 'descend' }
    return
  }
  sorter.value = {
    key: sorterInfo.columnKey,
    order: sorterInfo.order
  }
}

const handleSearch = () => {
  filters.value.host = filters.value.host.replace(/\s/g, '')
  filters.value.sourceIP = filters.value.sourceIP.replace(/\s/g, '')
  page.value = 1
  refetch()
}

const handleReset = () => {
  filters.value = {
    host: '',
    sourceIP: '',
    chain: '',
    startDate: null,
    endDate: null
  }
  timeRange.value = null
  sorter.value = {
    key: 'start',
    order: 'descend'
  }
  page.value = 1
  refetch()
}

const columns = [
  {
    title: 'Host',
    key: 'host',
    sorter: true,
    width: 300,
    render(row: any) {
      const host = row.host || ''
      return h(
        'span',
        {
          style: {
            cursor: 'pointer'
          },
          onClick: () => {
            filters.value.host = host
            handleSearch()
          }
        },
        host
      )
    }
  },
  {
   title: 'Source IP',
   key: 'sourceIP',
   sorter: true,
   width: 150,
   render(row: any) {
    const sourceIP = row.sourceIP || ''
    return h(
    	'span',
    	{
    		style: {
    			cursor: 'pointer'
    		},
    		onClick: () => {
    			filters.value.sourceIP = sourceIP
    			handleSearch()
    		}
    	},
    	sourceIP
    )
   }
  },
  {
    title: 'Upload',
    key: 'upload',
    sorter: true,
    width: 120,
    render(row: any) {
      return formatBytes(row.upload)
    }
  },
  {
    title: 'Download',
    key: 'download',
    sorter: true,
    width: 120,
    render(row: any) {
      return formatBytes(row.download)
    }
  },
  {
    title: 'Chain',
    key: 'chains',
    width: 150,
    render(row: any) {
      const chains = row.chains || []
      return h(
        'div',
        {},
        chains.map((chain: string) =>
          h(
            'span',
            {
              style: {
                cursor: 'pointer',
                marginRight: '8px'
              },
              onClick: () => {
                filters.value.chain = chain
                handleSearch()
              }
            },
            chain
          )
        )
      )
    }
  },
  {
    title: 'Start Time',
    key: 'start',
    sorter: true,
    width: 180,
    render(row: any) {
      return dayjs(row.start).format('YYYY/M/D HH:mm:ss')
    }
  }
]
const { mutate: mergeConnections, isPending: isMerging } = useMutation({
 mutationFn: (data: any) => axios.post('/api/connections/merge', data),
 onSuccess: () => {
   message.success('合并成功')
   showMergeModal.value = false
   refetch()
 },
 onError: (error: any) => {
   message.error(`合并失败: ${error.response?.data?.message || error.message}`)
 }
})

const { mutate: replaceHost, isPending: isReplacingHost } = useMutation({
 mutationFn: (data: any) => axios.post('/api/connections/replace-host', data),
 onSuccess: (response: any) => {
   message.success(`替换成功，影响了 ${response.data.rowsAffected} 条记录`)
   showReplaceHostModal.value = false
   refetch()
 },
 onError: (error: any) => {
   message.error(`替换失败: ${error.response?.data?.message || error.message}`)
 }
})

const handleMerge = () => {
  if (!mergeTimeRange.value) {
    message.error('请选择时间范围')
    return false
  }

  dialog.warning({
    title: '确认合并',
    content: '此操作将合并指定时间范围内的连接记录，以减少数据库体积。这是一个不可逆的操作，原始数据将被归档。',
    positiveText: '确认',
    negativeText: '取消',
    onPositiveClick: () => {
      mergeConnections({
        startDate: dayjs(mergeTimeRange.value![0]).unix(),
        endDate: dayjs(mergeTimeRange.value![1]).unix(),
        interval: mergeInterval.value
      })
    }
  })
  return false
}

const handleReplaceHost = () => {
 if (!replaceDomainSuffix.value) {
   message.error('请输入域名后缀')
   return false
 }

 dialog.warning({
   title: '确认替换',
   content: `确定要将所有 *.${replaceDomainSuffix.value} 的域名替换为 ${replaceDomainSuffix.value} 吗？这是一个不可逆的操作。`,
   positiveText: '确认',
   negativeText: '取消',
   onPositiveClick: () => {
     replaceHost({
       domainSuffix: replaceDomainSuffix.value
     })
   }
 })
}

const chartData = computed(() => {
  const labels = data.value?.data.map((item: any) => dayjs(item.start).format('HH:mm:ss')).reverse() || []
  const uploadData = data.value?.data.map((item: any) => item.upload).reverse() || []
  const downloadData = data.value?.data.map((item: any) => item.download).reverse() || []

  return {
    labels,
    datasets: [
      {
        label: '上传',
        data: uploadData,
        borderColor: '#74b9ff',
        backgroundColor: 'rgba(116, 185, 255, 0.1)',
        tension: 0.4,
        fill: true
      },
      {
        label: '下载',
        data: downloadData,
        borderColor: '#00b894',
        backgroundColor: 'rgba(0, 184, 148, 0.1)',
        tension: 0.4,
        fill: true
      }
    ]
  }
})

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  scales: {
    y: {
      ticks: {
        callback: function (value: any) {
          return formatBytes(value)
        }
      }
    }
  }
}
</script>