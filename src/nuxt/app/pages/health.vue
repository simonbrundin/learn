<template>
  <div class="container">
    <h1>Health Check</h1>
    
    <div v-if="loading" class="loading">Loading...</div>
    
    <div v-else-if="error" class="error">
      <p>Error: {{ error }}</p>
      <button @click="fetchHealth">Retry</button>
    </div>
    
    <div v-else-if="healthData" class="health-data">
      <p><strong>Status:</strong> {{ healthData.status }}</p>
      <p><strong>Version:</strong> {{ healthData.version || 'N/A' }}</p>
      <p><strong>Timestamp:</strong> {{ healthData.timestamp }}</p>
      <button @click="fetchHealth">Refresh</button>
    </div>
  </div>
</template>

<script setup lang="ts">
const loading = ref(true)
const error = ref<string | null>(null)
const healthData = ref<{
  status: string
  version?: string
  timestamp: string
} | null>(null)

async function fetchHealth() {
  loading.value = true
  error.value = null
  
  try {
    const response = await fetch('/api/health')
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`)
    }
    healthData.value = await response.json()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Unknown error'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchHealth()
})
</script>

<style scoped>
.container {
  max-width: 600px;
  margin: 2.4rem auto;
  padding: 1.2rem 1.3rem;
  background: #1a1e1b;
  border: 1px solid #2a312b;
  border-radius: 4px;
  color: #f0efe9;
  font-family: system-ui, sans-serif;
}

h1 {
  margin: 0;
  font-family: Georgia, 'Iowan Old Style', serif;
  font-weight: 400;
  color: #f0efe9;
}

.loading {
  color: #99a096;
}

.error {
  color: #e2a582;
  background: rgba(201, 127, 90, 0.1);
  border: 1px solid rgba(201, 127, 90, 0.35);
  padding: 1rem;
  border-radius: 4px;
}

.health-data {
  background: #212722;
  padding: 1rem;
  border-radius: 4px;
  border: 1px solid #2a312b;
}

.health-data p {
  margin: 0.5rem 0;
}

button {
  margin-top: 1rem;
  padding: 0.5rem 1rem;
  background: #c97f5a;
  color: #16100c;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: transform 120ms cubic-bezier(0.2, 0, 0, 1), background-color 120ms;
}

button:hover {
  background: #e2a582;
  transform: translateY(-1px);
}
</style>
