<script lang="ts">
  import { onMount } from 'svelte';
  import { slide } from 'svelte/transition';

  let inputText = '';
  
  let isLoading = false;
  let hasResult = false;
  let isAIEnabled = false;
  
  let result = {
    mappedRepos: [] as string[],
    tasks: [] as any[]
  };

  let assigneesList: string[] = ['Eddie', 'Antigravity'];
  let currentTaskGroupId = '';

  let isMockResponse = false;

  let isDragging = false;
  let uploadedFileName = '';
  let uploadedFileSize = 0;
  let showPromptSettings = false;
  let isImporting = false;

  let activeDemands: any[] = [];
  let selectedDemandId = '';
  let showDemandDropdown = false;
  let selectedDemandTitle = '选择要关联的产品需求 (可选)';

  function getDemandDisplayTitle(demand: any) {
    return `#${demand.task_id} - ${demand.title}`;
  }

  function createBrainGroupId(taskId: string) {
    const clean = taskId.replace(/[^A-Za-z0-9-]/g, '').toLowerCase();
    return clean ? `brain-${clean}` : `brain-${Date.now()}`;
  }

  function getDemandTaskGroupId(demand: any) {
    if (!demand || !demand.task_group_id || demand.task_group_id === '-') return '';
    return demand.task_group_id;
  }

  function getSelectedDemandTaskGroupId() {
    const selected = activeDemands.find((d: any) => d.task_id === selectedDemandId);
    return getDemandTaskGroupId(selected);
  }

  function selectDemandLink(demand: any | null) {
    if (!demand) {
      selectedDemandId = '';
      selectedDemandTitle = '不关联需求，仅同步任务';
      showDemandDropdown = false;
      return;
    }

    selectedDemandId = demand.task_id;
    selectedDemandTitle = getDemandDisplayTitle(demand);
    const existingGroupId = getDemandTaskGroupId(demand);
    if (existingGroupId) {
      currentTaskGroupId = existingGroupId;
    } else if (!currentTaskGroupId && hasResult) {
      currentTaskGroupId = createBrainGroupId(demand.task_id);
    }
    showDemandDropdown = false;
  }

  async function fetchActiveDemands() {
    try {
      const res = await fetch('/api/tasks');
      if (res.ok) {
        const data = await res.json();
        activeDemands = (data || []).filter((t: any) => t.issue_type === 'demand' && t.status !== 'archived');
      }
    } catch (e) {
      console.error('[Deconstructor] Failed to load active demands:', e);
    }
  }

  // 自定义下拉菜单状态控制
  let activeDropdown: { taskId: string; field: string } | null = null;

  function toggleDropdown(taskId: string, field: string) {
    if (activeDropdown && activeDropdown.taskId === taskId && activeDropdown.field === field) {
      activeDropdown = null;
    } else {
      activeDropdown = { taskId, field };
    }
  }

  function isDropdownOpen(taskId: string, field: string) {
    return activeDropdown && activeDropdown.taskId === taskId && activeDropdown.field === field;
  }

  function selectDropdownValue(taskId: string, field: string, value: any) {
    result.tasks = result.tasks.map(t => {
      if (t.id === taskId) {
        return { ...t, [field]: value };
      }
      return t;
    });
    activeDropdown = null;
  }

  function getPeriodLabel(days: number) {
    if (days === 1) return '1 天 (极速)';
    if (days === 2) return '2 天';
    if (days === 3) return '3 天 (快捷)';
    if (days === 5) return '5 天 (常规一周)';
    if (days === 7) return '7 天';
    if (days === 10) return '10 天 (双周)';
    if (days === 14) return '14 天 (长周期)';
    return `${days} 天`;
  }

  async function fetchConfig() {
    try {
      const res = await fetch('/api/config');
      if (res.ok) {
        const config = await res.json();
        isAIEnabled = !!(config.ai && config.ai.enabled);
        
        let users: string[] = [];
        if (config.jira) {
          if (config.jira.sync_users && config.jira.sync_users.length > 0) {
            users = [...config.jira.sync_users];
          } else if (config.jira.custom_jql) {
            const match = config.jira.custom_jql.match(/assignee\s+in\s*\(([^)]+)\)/i);
            if (match && match[1]) {
              users = match[1].split(',').map((name: string) => name.trim().replace(/['"]/g, ''));
            }
          }
        }
        if (users.length > 0) {
          assigneesList = users.filter(name => name !== '未指派' && name !== '-');
          console.log('[Deconstructor] Loaded team members from config:', assigneesList);
        } else {
          assigneesList = ['Eddie', 'Antigravity'];
        }
      }
    } catch (e) {
      console.error('[Deconstructor] Failed to load config:', e);
    }
  }

  function handleConfigUpdated(e: Event) {
    const customEvent = e as CustomEvent;
    const config = customEvent.detail;
    console.log('[Deconstructor] Received config-updated event:', config);
    if (config) {
      isAIEnabled = !!(config.ai && config.ai.enabled);
      
      let users: string[] = [];
      if (config.jira) {
        if (config.jira.sync_users && config.jira.sync_users.length > 0) {
          users = [...config.jira.sync_users];
        } else if (config.jira.custom_jql) {
          const match = config.jira.custom_jql.match(/assignee\s+in\s*\(([^)]+)\)/i);
          if (match && match[1]) {
            users = match[1].split(',').map((name: string) => name.trim().replace(/['"]/g, ''));
          }
        }
      }
      if (users.length > 0) {
        assigneesList = users.filter(name => name !== '未指派' && name !== '-');
        console.log('[Deconstructor] Hot-updated team members from config-updated:', assigneesList);
      } else {
        assigneesList = ['Eddie', 'Antigravity'];
      }
      console.log('[Deconstructor] Updated isAIEnabled to =', isAIEnabled);
    }
  }

  onMount(() => {
    fetchConfig();
    fetchActiveDemands();
    
    const handleWindowFocus = () => {
      fetchConfig();
      fetchActiveDemands();
    };

    const handleOutsideClick = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      if (activeDropdown && !target.closest('.custom-dropdown-container')) {
        activeDropdown = null;
      }
      if (showDemandDropdown && !target.closest('.deconstruct-demand-link-select')) {
        showDemandDropdown = false;
      }
    };

    if (typeof window !== 'undefined') {
      window.addEventListener('config-updated', handleConfigUpdated);
      window.addEventListener('focus', handleWindowFocus);
      window.addEventListener('click', handleOutsideClick);
    }
    return () => {
      if (typeof window !== 'undefined') {
        window.removeEventListener('config-updated', handleConfigUpdated);
        window.removeEventListener('focus', handleWindowFocus);
        window.removeEventListener('click', handleOutsideClick);
      }
    };
  });

  async function handleDeconstruct() {
    if (!inputText.trim()) {
      displayToast('请先输入要解构的需求描述', 'error');
      return;
    }

    isLoading = true;
    hasResult = false;
    isMockResponse = false;
    currentTaskGroupId = '';

    try {
      const res = await fetch('/api/deconstruct', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ text: inputText })
      });

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || `HTTP 错误 ${res.status}`);
      }

      const data = await res.json();
      const rawTasks = data.tasks || [];
      
      // 规范化负责人：模糊匹配真实 sync_users 列表中的名字，不符合的强制指派
      const tasksWithDefaults = rawTasks.map((t: any) => {
        let assigned = t.assignee || '';
        let matchedMember = '';
        for (const member of assigneesList) {
          if (assigned.toLowerCase().includes(member.toLowerCase())) {
            matchedMember = member;
            break;
          }
        }
        
        let finalAssignee = '';
        if (matchedMember) {
          finalAssignee = matchedMember;
        } else {
          // 如果大模型随机生成的负责人不属于实际筛选人员名单，强制匹配到第一个真实团队人员
          finalAssignee = assigneesList.length > 0 ? assigneesList[0] : 'Unassigned';
        }

        return {
          ...t,
          assignee: finalAssignee,
          period_days: t.period_days || 5
        };
      });

      result = {
        mappedRepos: data.mappedRepos || [],
        tasks: tasksWithDefaults
      };
      
      // 成功生成解构任务时，优先沿用已选需求的任务组 ID，保证二次解构仍挂在同一父需求上。
      currentTaskGroupId = getSelectedDemandTaskGroupId() || (selectedDemandId ? createBrainGroupId(selectedDemandId) : 'group-' + Date.now());
      isMockResponse = !!data.is_mock;
      hasResult = true;
    } catch (e: any) {
      console.error('AI Deconstruct failed:', e);
      displayToast(`AI 解构失败: ${e.message}`, 'error');
    } finally {
      isLoading = false;
    }
  }

  let toastMsg = '';
  let toastType: 'success' | 'error' | 'info' = 'info';
  let showToast = false;
  let toastTimeout: any;

  function displayToast(msg: string, type: 'success' | 'error' | 'info' = 'info') {
    toastMsg = msg;
    toastType = type;
    showToast = true;
    if (toastTimeout) clearTimeout(toastTimeout);
    toastTimeout = setTimeout(() => {
      showToast = false;
    }, 3000);
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    if (!isAIEnabled) return;
    isDragging = true;
  }

  function handleDragLeave() {
    isDragging = false;
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault();
    isDragging = false;
    if (!isAIEnabled) return;
    const files = e.dataTransfer?.files;
    if (files && files.length > 0) {
      processFile(files[0]);
    }
  }

  function handleFileSelect(e: Event) {
    const input = e.target as HTMLInputElement;
    const files = input.files;
    if (files && files.length > 0) {
      processFile(files[0]);
    }
  }

  function processFile(file: File) {
    const allowedExtensions = ['.txt', '.md', '.json', '.csv', '.xml', '.html'];
    const fileName = file.name.toLowerCase();
    const isAllowed = allowedExtensions.some(ext => fileName.endsWith(ext));

    if (!isAllowed) {
      displayToast('仅支持文本类文件 (如 .txt, .md, .json 等)', 'error');
      return;
    }

    uploadedFileName = file.name;
    uploadedFileSize = file.size;

    const reader = new FileReader();
    reader.onload = (e) => {
      const text = e.target?.result as string;
      inputText = text;
      displayToast(`已成功读取文档 "${file.name}" 并填充到需求描述中。`, 'success');
    };
    reader.readAsText(file);
  }

  async function importTasksToKanban() {
    if (!result.tasks || result.tasks.length === 0) return;
    isImporting = true;
    try {
      const res = await fetch('/api/tasks/import', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ 
          task_group_id: currentTaskGroupId,
          demand_id: selectedDemandId,
          tasks: result.tasks 
        })
      });

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || `HTTP ${res.status}`);
      }

      await fetchActiveDemands();
      displayToast(`成功同步 ${result.tasks.length} 个影子任务至项目看板！`, 'success');
    } catch (e: any) {
      console.error('Import tasks failed:', e);
      displayToast(`同步失败: ${e.message}`, 'error');
    } finally {
      isImporting = false;
    }
  }

  function deleteTask(taskId: string) {
    result.tasks = result.tasks.filter(t => t.id !== taskId);
    displayToast('任务卡片已删除', 'info');
  }
</script>

<section class="deconstructor-section">
  <div class="section-header">
    <div class="header-left">
      <h2 class="section-title">需求解构引擎</h2>
      <span class="badge">Deconstructor</span>
    </div>
    <span class="badge-sub">对接 LLM 自动映射多仓</span>
  </div>

  <div class="split-layout">
    <!-- Input Panel -->
    <div class="panel input-panel {!isAIEnabled ? 'disabled-panel' : ''}">
      <label for="raw-demand" class="input-label">输入非结构化需求草案 / Bug 描述</label>
      
      {#if !isAIEnabled}
        <div class="ai-disabled-indicator">
          <span class="warning-icon">⚠️</span>
          <div class="warning-text">
            <strong>AI 需求自解构已锁定</strong>
            <p>由于后台大模型配置未启用或未检测到 API 凭证，本功能已禁用。请在上方“集成状态中枢”中配置并启用大模型服务。</p>
          </div>
        </div>
      {/if}

      <!-- File Upload Dropzone -->
      {#if isAIEnabled}
        <div 
          class="file-dropzone {isDragging ? 'dragging' : ''}" 
          on:dragover={handleDragOver}
          on:dragleave={handleDragLeave}
          on:drop={handleDrop}
          role="button"
          tabindex="0"
        >
          <input 
            type="file" 
            id="file-upload" 
            accept=".txt,.md,.json,.csv,.xml,.html" 
            on:change={handleFileSelect} 
            class="file-input"
          />
          <label for="file-upload" class="dropzone-label">
            <span class="upload-icon">📂</span>
            {#if uploadedFileName}
              <span class="upload-text text-indigo">已加载：{uploadedFileName} ({uploadedFileSize} 字节)</span>
            {:else}
              <span class="upload-text">拖拽需求文档 (.md / .txt) 至此 或 <span class="browse-link">浏览文件</span></span>
            {/if}
          </label>
        </div>
      {/if}

      <textarea
        id="raw-demand"
        bind:value={inputText}
        disabled={!isAIEnabled}
        class="demand-textarea"
        placeholder={isAIEnabled ? "在此粘贴或上传原始需求描述..." : "AI 服务未启用，不可输入..."}
      ></textarea>

      <!--折叠 Prompt 约束 -->
      {#if isAIEnabled}
        <div class="prompt-constraint-panel">
          <button class="prompt-toggle-btn" on:click={() => showPromptSettings = !showPromptSettings}>
            <span>⚙️ 解构 Prompt 约束词规约</span>
            <span class="arrow">{showPromptSettings ? '▲' : '▼'}</span>
          </button>
          {#if showPromptSettings}
            <div class="prompt-content font-mono" transition:slide>
              <p class="prompt-desc">解构引擎将固定使用系统 Prompt 对大模型进行强规约，确保输出的任务严格匹配代码仓和分配逻辑，保障入库格式的契约一致性。</p>
              <pre class="prompt-pre"><code>{`【输出 JSON 契约规约】
{
  "mappedRepos": ["仓库名称"],
  "tasks": [
    {
      "id": "task-xxx",
      "repo": "仓库名称",
      "title": "任务标题（描述该仓库具体功能）",
      "assignee": "推荐人名字（如 Eddie）",
      "priority": "High/Medium/Low",
      "complexity": "High/Medium/Low"
    }
  ]
}`}</code></pre>
            </div>
          {/if}
        </div>
      {/if}
      
      <div class="actions">
        <button
          on:click={handleDeconstruct}
          disabled={isLoading || !isAIEnabled}
          class="btn-deconstruct"
        >
          {#if isLoading}
            <span class="spinner-small"></span>
            解构分析中...
          {:else}
            🚀 需求解构
          {/if}
        </button>
      </div>
    </div>

    <!-- Output Panel -->
    <div class="panel output-panel">
      {#if !isLoading && !hasResult}
        <div class="empty-state">
          <p class="empty-main">输入左侧需求并点击“需求解构”</p>
          <p class="empty-sub">AI 将为您提取最关联代码库并生成影子任务卡</p>
        </div>
      {/if}

      {#if isLoading}
        <div class="loading-state">
          <div class="spinner-large"></div>
          <p class="loading-text">检索仓库元数据，匹配 Project-Mapping...</p>
        </div>
      {/if}

      {#if hasResult}
        <div class="result-content">

          <div class="result-section">
            <span class="result-section-label">匹配目标仓库 (Project-Mapping)</span>
            <div class="repo-list">
              {#each result.mappedRepos as repo}
                <span class="repo-badge">
                  📁 {repo}
                </span>
              {/each}
            </div>
          </div>

          <div class="result-section">
            <div class="result-header-row">
              <span class="result-section-label">生成影子任务卡 (Subtasks)</span>
              
              <div class="sync-actions-group">
                <!-- 关联需求下拉框 -->
                <div class="custom-dropdown-container deconstruct-demand-link-select">
                  <button 
                    type="button"
                    class="dropdown-trigger" 
                    on:click|stopPropagation={() => showDemandDropdown = !showDemandDropdown}
                  >
                    <span>{selectedDemandId === '' ? '不关联需求，仅同步任务' : selectedDemandTitle}</span>
                    <span class="arrow-icon {showDemandDropdown ? 'open' : ''}">▼</span>
                  </button>
                  {#if showDemandDropdown}
                    <div class="dropdown-options-list glass-panel">
                      <button 
                        type="button"
                        class="dropdown-option-item {selectedDemandId === '' ? 'selected' : ''}"
                        on:click={() => selectDemandLink(null)}
                      >
                        不关联需求，仅同步任务
                      </button>
                      {#each activeDemands as d}
                        <button 
                          type="button"
                          class="dropdown-option-item {selectedDemandId === d.task_id ? 'selected' : ''}"
                          on:click={() => selectDemandLink(d)}
                        >
                          {getDemandDisplayTitle(d)}
                        </button>
                      {/each}
                    </div>
                  {/if}
                </div>

                <button 
                  class="btn-sync-kanban font-sans" 
                  on:click={importTasksToKanban} 
                  disabled={isImporting}
                >
                  {#if isImporting}
                    <span class="spinner-small"></span> 同步中...
                  {:else}
                    📥 一键同步至看板
                  {/if}
                </button>
              </div>
            </div>
            <div class="task-list">
              {#each result.tasks as task (task.id)}
                <div 
                  class="generated-task-card"
                  style={activeDropdown && activeDropdown.taskId === task.id ? 'z-index: 10;' : 'z-index: 1;'}
                >
                  <!-- 卡片顶部栏 -->
                  <div class="task-card-top-row">
                    <div class="task-meta-group">
                      <span class="task-gen-id">{task.id}</span>
                      {#if currentTaskGroupId}
                        <span class="task-group-badge" title="当前影子任务所属的大脑任务组">🔗 {currentTaskGroupId}</span>
                      {/if}
                    </div>
                    
                    <button class="delete-task-btn-premium" on:click={() => deleteTask(task.id)} title="删除影子任务">
                      <svg xmlns="http://www.w3.org/2000/svg" class="icon-trash-premium" viewBox="0 0 20 20" fill="currentColor">
                        <path fill-rule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clip-rule="evenodd" />
                      </svg>
                      <span class="delete-text-mini">删除</span>
                    </button>
                  </div>
                  
                  <!-- 卡片标题编辑 -->
                  <div class="task-title-container">
                    <span class="edit-icon-indicator">✏️</span>
                    <input type="text" class="task-title-input-premium" bind:value={task.title} placeholder="修改任务标题..." />
                  </div>
                  
                  <!-- Bento 风格的开发属性微调网格 -->
                  <div class="task-bento-grid">
                    <!-- 关联仓库 -->
                    <div class="bento-edit-cell">
                      <span class="cell-label">📦 关联仓库</span>
                      <div class="custom-dropdown-container">
                        <button 
                          type="button"
                          class="custom-dropdown-trigger" 
                          on:click|stopPropagation={() => toggleDropdown(task.id, 'repo')}
                        >
                          <span class="trigger-value">{task.repo}</span>
                          <span class="trigger-arrow {isDropdownOpen(task.id, 'repo') ? 'rotated' : ''}">▼</span>
                        </button>
                        
                        {#if isDropdownOpen(task.id, 'repo')}
                          <div class="custom-dropdown-options" transition:slide={{ duration: 150 }}>
                            {#each result.mappedRepos.length > 0 ? result.mappedRepos : ['frontend-dashboard', 'backend-core'] as repo}
                              <button 
                                type="button"
                                class="dropdown-option-btn {task.repo === repo ? 'selected' : ''}" 
                                on:click|stopPropagation={() => selectDropdownValue(task.id, 'repo', repo)}
                              >
                                {repo}
                              </button>
                            {/each}
                          </div>
                        {/if}
                      </div>
                    </div>
                    
                    <!-- 负责人 -->
                    <div class="bento-edit-cell">
                      <span class="cell-label">👤 负责人</span>
                      <div class="custom-dropdown-container">
                        <button 
                          type="button"
                          class="custom-dropdown-trigger" 
                          on:click|stopPropagation={() => toggleDropdown(task.id, 'assignee')}
                        >
                          <span class="trigger-value">{task.assignee}</span>
                          <span class="trigger-arrow {isDropdownOpen(task.id, 'assignee') ? 'rotated' : ''}">▼</span>
                        </button>
                        
                        {#if isDropdownOpen(task.id, 'assignee')}
                          <div class="custom-dropdown-options" transition:slide={{ duration: 150 }}>
                            {#each assigneesList as member}
                              <button 
                                type="button"
                                class="dropdown-option-btn {task.assignee === member ? 'selected' : ''}" 
                                on:click|stopPropagation={() => selectDropdownValue(task.id, 'assignee', member)}
                              >
                                {member}
                              </button>
                            {/each}
                            <button 
                              type="button"
                              class="dropdown-option-btn {task.assignee === 'Unassigned' ? 'selected' : ''}" 
                              on:click|stopPropagation={() => selectDropdownValue(task.id, 'assignee', 'Unassigned')}
                            >
                              暂无分配
                            </button>
                          </div>
                        {/if}
                      </div>
                    </div>
                    
                    <!-- 开发周期 -->
                    <div class="bento-edit-cell">
                      <span class="cell-label">⏳ 开发周期</span>
                      <div class="custom-dropdown-container">
                        <button 
                          type="button"
                          class="custom-dropdown-trigger" 
                          on:click|stopPropagation={() => toggleDropdown(task.id, 'period_days')}
                        >
                          <span class="trigger-value">{getPeriodLabel(task.period_days)}</span>
                          <span class="trigger-arrow {isDropdownOpen(task.id, 'period_days') ? 'rotated' : ''}">▼</span>
                        </button>
                        
                        {#if isDropdownOpen(task.id, 'period_days')}
                          <div class="custom-dropdown-options" transition:slide={{ duration: 150 }}>
                            {#each [1, 2, 3, 5, 7, 10, 14] as days}
                              <button 
                                type="button"
                                class="dropdown-option-btn {task.period_days === days ? 'selected' : ''}" 
                                on:click|stopPropagation={() => selectDropdownValue(task.id, 'period_days', days)}
                              >
                                {getPeriodLabel(days)}
                              </button>
                            {/each}
                          </div>
                        {/if}
                      </div>
                    </div>
 
                    <!-- 优先级 -->
                    <div class="bento-edit-cell">
                      <span class="cell-label">⚡ 优先级</span>
                      <div class="custom-dropdown-container">
                        <button 
                          type="button"
                          class="custom-dropdown-trigger" 
                          on:click|stopPropagation={() => toggleDropdown(task.id, 'priority')}
                        >
                          <span class="trigger-value">{task.priority === 'High' ? 'High (高)' : task.priority === 'Medium' ? 'Medium (中)' : 'Low (低)'}</span>
                          <span class="trigger-arrow {isDropdownOpen(task.id, 'priority') ? 'rotated' : ''}">▼</span>
                        </button>
                        
                        {#if isDropdownOpen(task.id, 'priority')}
                          <div class="custom-dropdown-options" transition:slide={{ duration: 150 }}>
                            <button 
                              type="button"
                              class="dropdown-option-btn {task.priority === 'High' ? 'selected' : ''}" 
                              on:click|stopPropagation={() => selectDropdownValue(task.id, 'priority', 'High')}
                            >
                              High (高)
                            </button>
                            <button 
                              type="button"
                              class="dropdown-option-btn {task.priority === 'Medium' ? 'selected' : ''}" 
                              on:click|stopPropagation={() => selectDropdownValue(task.id, 'priority', 'Medium')}
                            >
                              Medium (中)
                            </button>
                            <button 
                              type="button"
                              class="dropdown-option-btn {task.priority === 'Low' ? 'selected' : ''}" 
                              on:click|stopPropagation={() => selectDropdownValue(task.id, 'priority', 'Low')}
                            >
                              Low (低)
                            </button>
                          </div>
                        {/if}
                      </div>
                    </div>
 
                    <!-- 复杂度 -->
                    <div class="bento-edit-cell">
                      <span class="cell-label">📊 复杂度</span>
                      <div class="custom-dropdown-container">
                        <button 
                          type="button"
                          class="custom-dropdown-trigger" 
                          on:click|stopPropagation={() => toggleDropdown(task.id, 'complexity')}
                        >
                          <span class="trigger-value">{task.complexity === 'High' ? 'High (高)' : task.complexity === 'Medium' ? 'Medium (中)' : 'Low (低)'}</span>
                          <span class="trigger-arrow {isDropdownOpen(task.id, 'complexity') ? 'rotated' : ''}">▼</span>
                        </button>
                        
                        {#if isDropdownOpen(task.id, 'complexity')}
                          <div class="custom-dropdown-options" transition:slide={{ duration: 150 }}>
                            <button 
                              type="button"
                              class="dropdown-option-btn {task.complexity === 'High' ? 'selected' : ''}" 
                              on:click|stopPropagation={() => selectDropdownValue(task.id, 'complexity', 'High')}
                            >
                              High (高)
                            </button>
                            <button 
                              type="button"
                              class="dropdown-option-btn {task.complexity === 'Medium' ? 'selected' : ''}" 
                              on:click|stopPropagation={() => selectDropdownValue(task.id, 'complexity', 'Medium')}
                            >
                              Medium (中)
                            </button>
                            <button 
                              type="button"
                              class="dropdown-option-btn {task.complexity === 'Low' ? 'selected' : ''}" 
                              on:click|stopPropagation={() => selectDropdownValue(task.id, 'complexity', 'Low')}
                            >
                              Low (低)
                            </button>
                          </div>
                        {/if}
                      </div>
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          </div>
        </div>
      {/if}
    </div>
  </div>

  <div class="toast {toastType} {showToast ? 'show' : ''}">
    {#if toastType === 'success'}
      <span>✅</span>
    {:else if toastType === 'error'}
      <span>❌</span>
    {:else}
      <span>ℹ️</span>
    {/if}
    <span>{toastMsg}</span>
  </div>
</section>

<style>
  .deconstructor-section {
    background: rgba(17, 24, 39, 0.6);
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 12px;
    padding: 24px;
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    margin-bottom: 32px;
    box-sizing: border-box;
  }

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 24px;
    flex-wrap: wrap;
    gap: 12px;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .section-title {
    font-size: 1.5rem;
    font-weight: 700;
    background: linear-gradient(135deg, #e2e8f0 0%, #cbd5e1 50%, #e2e8f0 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    margin: 0;
  }

  .badge {
    font-size: 0.75rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    background: rgba(99, 102, 241, 0.1);
    color: #818cf8;
    border: 1px solid rgba(99, 102, 241, 0.2);
    padding: 2px 8px;
    border-radius: 4px;
  }

  .badge-sub {
    font-size: 0.75rem;
    color: #64748b;
  }

  .split-layout {
    display: grid;
    grid-template-columns: repeat(1, minmax(0, 1fr));
    gap: 32px;
  }

  @media (min-width: 1024px) {
    .split-layout {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  .panel {
    display: flex;
    flex-direction: column;
    box-sizing: border-box;
  }

  .disabled-panel {
    position: relative;
  }

  .demand-textarea:disabled {
    background: rgba(2, 6, 23, 0.4);
    border-color: rgba(51, 65, 85, 0.2);
    color: #475569;
    cursor: not-allowed;
  }

  .ai-disabled-indicator {
    display: flex;
    gap: 12px;
    background: rgba(239, 68, 68, 0.08);
    border: 1px solid rgba(239, 68, 68, 0.2);
    padding: 12px;
    border-radius: 8px;
    margin-bottom: 12px;
    align-items: flex-start;
  }

  .ai-disabled-indicator .warning-icon {
    font-size: 1.1rem;
    line-height: 1;
  }

  .ai-disabled-indicator .warning-text strong {
    font-size: 0.8rem;
    font-weight: 700;
    color: #f87171;
    display: block;
    margin-bottom: 4px;
  }

  .ai-disabled-indicator .warning-text p {
    font-size: 0.75rem;
    color: #94a3b8;
    margin: 0;
    line-height: 1.4;
  }

  .input-label {
    font-size: 0.875rem;
    font-weight: 500;
    color: #94a3b8;
    margin-bottom: 8px;
  }

  .demand-textarea {
    width: 100%;
    height: 192px;
    background: rgba(2, 6, 23, 0.8);
    border: 1px solid rgba(51, 65, 85, 0.4);
    border-radius: 8px;
    padding: 16px;
    color: #cbd5e1;
    font-size: 0.875rem;
    line-height: 1.6;
    font-family: system-ui, -apple-system, sans-serif;
    resize: none;
    box-sizing: border-box;
    transition: border-color 0.2s;
  }

  .demand-textarea:focus {
    outline: none;
    border-color: #6366f1;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 16px;
  }

  .btn-deconstruct {
    background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
    color: #ffffff;
    border: none;
    border-radius: 8px;
    padding: 10px 20px;
    font-size: 0.875rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    display: inline-flex;
    align-items: center;
    gap: 8px;
    box-shadow: 0 4px 12px rgba(79, 70, 229, 0.2);
  }

  .btn-deconstruct:hover {
    background: linear-gradient(135deg, #4338ca 0%, #6d28d9 100%);
    box-shadow: 0 4px 16px rgba(79, 70, 229, 0.35);
    transform: translateY(-1px);
  }

  .btn-deconstruct:disabled {
    opacity: 0.6;
    cursor: not-allowed;
    transform: none;
  }

  .output-panel {
    background: rgba(2, 6, 23, 0.2);
    border: 1px solid rgba(51, 65, 85, 0.25);
    border-radius: 8px;
    padding: 20px;
    justify-content: center;
    min-height: 250px;
  }

  .empty-state {
    text-align: center;
    padding: 32px 0;
  }

  .empty-main {
    font-size: 0.875rem;
    color: #64748b;
    margin: 0 0 6px 0;
  }

  .empty-sub {
    font-size: 0.75rem;
    color: #475569;
    margin: 0;
  }

  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 48px 0;
  }

  .loading-text {
    font-size: 0.875rem;
    color: #94a3b8;
    margin-top: 16px;
    font-weight: 500;
  }

  .spinner-small {
    width: 16px;
    height: 16px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-top-color: #ffffff;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    display: inline-block;
  }

  .spinner-large {
    width: 32px;
    height: 32px;
    border: 3px solid rgba(99, 102, 241, 0.2);
    border-top-color: #6366f1;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .result-content {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .result-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .result-section-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: #64748b;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .repo-list {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .repo-badge {
    padding: 4px 10px;
    background: #0f172a;
    border: 1px solid rgba(99, 102, 241, 0.2);
    border-radius: 4px;
    font-size: 0.75rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    color: #818cf8;
    font-weight: 500;
  }

  .task-list {
    display: flex;
    flex-direction: column;
    gap: 16px;
    max-height: 480px;
    overflow-y: auto;
    padding-right: 4px;
  }

  .task-list::-webkit-scrollbar {
    width: 4px;
  }

  .task-list::-webkit-scrollbar-thumb {
    background: rgba(148, 163, 184, 0.2);
    border-radius: 3px;
  }

  .generated-task-card {
    background: rgba(15, 23, 42, 0.45);
    border: 1px solid rgba(51, 65, 85, 0.35);
    padding: 16px;
    border-radius: 10px;
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
    box-shadow: 0 4px 20px -2px rgba(0, 0, 0, 0.3);
    position: relative;
    overflow: visible;
  }

  .generated-task-card::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 1px;
    background: linear-gradient(90deg, transparent, rgba(56, 189, 248, 0.2), transparent);
  }

  .generated-task-card:hover {
    border-color: rgba(56, 189, 248, 0.35);
    box-shadow: 0 8px 30px -4px rgba(0, 0, 0, 0.4), 0 0 15px rgba(56, 189, 248, 0.06);
    transform: translateY(-2px);
  }

  .task-card-top-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }

  .delete-task-btn-premium {
    background: rgba(239, 68, 68, 0.08);
    border: 1px solid rgba(239, 68, 68, 0.2);
    color: #f87171;
    border-radius: 6px;
    padding: 4px 8px;
    font-size: 0.68rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .delete-task-btn-premium:hover {
    background: rgba(239, 68, 68, 0.22);
    border-color: rgba(239, 68, 68, 0.45);
    color: #fca5a5;
    transform: scale(1.03);
  }

  .icon-trash-premium {
    width: 13px;
    height: 13px;
    transition: transform 0.2s;
  }

  .delete-task-btn-premium:hover .icon-trash-premium {
    transform: rotate(6deg) scale(1.05);
  }

  .delete-text-mini {
    font-family: inherit;
  }

  .task-title-container {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(2, 6, 23, 0.4);
    border: 1px solid rgba(51, 65, 85, 0.25);
    border-radius: 6px;
    padding: 6px 12px;
    margin-bottom: 14px;
    transition: all 0.2s;
  }

  .task-title-container:focus-within {
    border-color: rgba(56, 189, 248, 0.5);
    box-shadow: 0 0 8px rgba(56, 189, 248, 0.15);
    background: rgba(2, 6, 23, 0.6);
  }

  .edit-icon-indicator {
    font-size: 0.8rem;
    opacity: 0.6;
    transition: opacity 0.2s;
  }

  .task-title-container:focus-within .edit-icon-indicator {
    opacity: 1;
  }

  .task-title-input-premium {
    flex: 1;
    background: transparent;
    border: none;
    outline: none;
    color: #f1f5f9;
    font-size: 0.85rem;
    font-weight: 500;
    font-family: inherit;
    padding: 0;
    box-sizing: border-box;
  }

  .task-title-input-premium::placeholder {
    color: #475569;
  }

  .task-bento-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }

  @media (min-width: 640px) {
    .task-bento-grid {
      grid-template-columns: repeat(3, 1fr);
    }
  }

  .bento-edit-cell {
    background: rgba(15, 23, 42, 0.4);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 6px;
    padding: 6px 10px;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    transition: all 0.2s;
  }

  .bento-edit-cell:hover {
    border-color: rgba(51, 65, 85, 0.5);
    background: rgba(15, 23, 42, 0.55);
  }

  .cell-label {
    font-size: 0.62rem;
    color: #64748b;
    font-weight: 600;
    margin-bottom: 2px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  /* 任务组标识样式 */
  .task-meta-group {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .task-group-badge {
    font-size: 0.62rem;
    color: #38bdf8;
    background: rgba(56, 189, 248, 0.08);
    border: 1px solid rgba(56, 189, 248, 0.2);
    padding: 1px 5px;
    border-radius: 4px;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-weight: 600;
  }

  /* 自定义非原生下拉菜单样式 */
  .custom-dropdown-container {
    position: relative;
    width: 100%;
  }

  .custom-dropdown-trigger {
    width: 100%;
    background: rgba(15, 23, 42, 0.25);
    border: 1px solid rgba(51, 65, 85, 0.25);
    border-radius: 4px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    color: #cbd5e1;
    font-size: 0.7rem;
    font-weight: 500;
    padding: 3px 6px;
    cursor: pointer;
    text-align: left;
    transition: all 0.2s;
  }

  .custom-dropdown-trigger:hover, .custom-dropdown-trigger:focus {
    color: #f1f5f9;
    border-color: rgba(56, 189, 248, 0.4);
    background: rgba(15, 23, 42, 0.45);
  }

  .trigger-value {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    margin-right: 6px;
  }

  .trigger-arrow {
    font-size: 0.52rem;
    color: #64748b;
    transition: transform 0.2s, color 0.2s;
    line-height: 1;
  }

  .trigger-arrow.rotated {
    transform: rotate(180deg);
    color: #38bdf8;
  }

  .custom-dropdown-options {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    right: 0;
    background: #0d1527;
    border: 1px solid rgba(56, 189, 248, 0.25);
    border-radius: 6px;
    box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.6), 0 0 12px rgba(56, 189, 248, 0.1);
    z-index: 99;
    max-height: 160px;
    overflow-y: auto;
    padding: 4px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .custom-dropdown-options::-webkit-scrollbar {
    width: 5px;
  }

  .custom-dropdown-options::-webkit-scrollbar-thumb {
    background: rgba(56, 189, 248, 0.2);
    border-radius: 3px;
  }

  .dropdown-option-btn {
    width: 100%;
    background: transparent;
    border: none;
    outline: none;
    color: #94a3b8;
    font-size: 0.68rem;
    padding: 5px 8px;
    text-align: left;
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.15s;
  }

  .dropdown-option-btn:hover {
    background: rgba(56, 189, 248, 0.08);
    color: #f1f5f9;
  }

  .dropdown-option-btn.selected {
    background: rgba(56, 189, 248, 0.15);
    color: #38bdf8;
    font-weight: 600;
  }

  /* Toast Notification */
  .toast {
    position: fixed;
    bottom: 24px;
    right: 24px;
    z-index: 50;
    padding: 12px 20px;
    border-radius: 8px;
    font-size: 0.85rem;
    font-weight: 500;
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5);
    border: 1px solid;
    display: flex;
    align-items: center;
    gap: 8px;
    transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
    transform: translateY(100px);
    opacity: 0;
  }

  .toast.show {
    transform: translateY(0);
    opacity: 1;
  }

  .toast.success {
    background: rgba(16, 185, 129, 0.15);
    border-color: rgba(16, 185, 129, 0.4);
    color: #34d399;
  }

  .toast.error {
    background: rgba(239, 68, 68, 0.15);
    border-color: rgba(239, 68, 68, 0.4);
    color: #f87171;
  }

  .toast.info {
    background: rgba(99, 102, 241, 0.15);
    border-color: rgba(99, 102, 241, 0.4);
    color: #818cf8;
  }

  .mock-alert-banner {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    background: rgba(245, 158, 11, 0.1);
    border: 1px solid rgba(245, 158, 11, 0.3);
    border-radius: 8px;
    padding: 12px 16px;
    margin-bottom: 20px;
    animation: fadeIn 0.3s ease-out;
  }

  .mock-alert-icon {
    font-size: 1.2rem;
    line-height: 1;
    margin-top: 2px;
  }

  .mock-alert-text-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
    text-align: left;
  }

  .mock-alert-title {
    font-size: 0.85rem;
    font-weight: 700;
    color: #f59e0b;
    margin: 0;
  }

  .mock-alert-desc {
    font-size: 0.75rem;
    color: #cbd5e1;
    margin: 0;
    line-height: 1.4;
  }

  /* File Upload Dropzone */
  .file-dropzone {
    border: 1px dashed rgba(99, 102, 241, 0.35);
    background: rgba(15, 23, 42, 0.4);
    border-radius: 8px;
    padding: 14px;
    text-align: center;
    cursor: pointer;
    transition: all 0.2s ease;
    margin-bottom: 12px;
    position: relative;
  }

  .file-dropzone:hover, .file-dropzone.dragging {
    border-color: #6366f1;
    background: rgba(30, 41, 59, 0.6);
    box-shadow: 0 0 12px rgba(99, 102, 241, 0.15);
  }

  .file-input {
    position: absolute;
    width: 100%;
    height: 100%;
    top: 0;
    left: 0;
    opacity: 0;
    cursor: pointer;
  }

  .dropzone-label {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    cursor: pointer;
    pointer-events: none;
  }

  .upload-icon {
    font-size: 1.1rem;
  }

  .upload-text {
    font-size: 0.78rem;
    color: #94a3b8;
  }

  .upload-text.text-indigo {
    color: #818cf8;
    font-weight: 600;
  }

  .browse-link {
    color: #6366f1;
    text-decoration: underline;
    font-weight: 500;
  }

  /* Prompt Constraint Panel */
  .prompt-constraint-panel {
    margin-top: 12px;
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 8px;
    background: rgba(15, 23, 42, 0.3);
    overflow: hidden;
  }

  .prompt-toggle-btn {
    width: 100%;
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: transparent;
    border: none;
    padding: 10px 14px;
    color: #94a3b8;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    transition: background 0.2s;
  }

  .prompt-toggle-btn:hover {
    background: rgba(255, 255, 255, 0.02);
    color: #e2e8f0;
  }

  .prompt-content {
    padding: 0 14px 14px 14px;
    border-top: 1px solid rgba(51, 65, 85, 0.15);
  }

  .prompt-desc {
    font-size: 0.7rem;
    color: #64748b;
    line-height: 1.4;
    margin: 8px 0;
  }

  .prompt-pre {
    margin: 6px 0 0 0;
    background: rgba(2, 6, 23, 0.6);
    border: 1px solid rgba(51, 65, 85, 0.3);
    border-radius: 4px;
    padding: 8px;
    overflow-x: auto;
    font-size: 0.68rem;
    color: #818cf8;
  }

  /* Result Header Row */
  .result-header-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  .sync-actions-group {
    display: flex;
    align-items: center;
    gap: 12px;
    position: relative;
  }

  .deconstruct-demand-link-select {
    width: 220px;
    font-size: 0.72rem;
  }

  .deconstruct-demand-link-select .dropdown-trigger {
    width: 100%;
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(129, 140, 248, 0.25);
    color: #cbd5e1;
    padding: 4px 10px;
    border-radius: 6px;
    cursor: pointer;
    display: flex;
    justify-content: space-between;
    align-items: center;
    user-select: none;
    transition: all 0.2s;
    font: inherit;
  }

  .deconstruct-demand-link-select .dropdown-trigger:hover {
    border-color: rgba(99, 102, 241, 0.5);
    background: rgba(30, 41, 59, 0.8);
  }

  .deconstruct-demand-link-select .dropdown-trigger span {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 190px;
  }

  .deconstruct-demand-link-select .dropdown-options-list {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    width: 100%;
    max-height: 180px;
    overflow-y: auto;
    background: #0f172a;
    border: 1px solid rgba(129, 140, 248, 0.25);
    border-radius: 8px;
    box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5);
    z-index: 120;
    padding: 4px;
    box-sizing: border-box;
  }

  .deconstruct-demand-link-select .dropdown-option-item {
    width: 100%;
    border: 0;
    background: transparent;
    padding: 6px 10px;
    color: #cbd5e1;
    border-radius: 4px;
    cursor: pointer;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    transition: all 0.15s;
    text-align: left;
  }

  .deconstruct-demand-link-select .dropdown-option-item:hover {
    background: rgba(99, 102, 241, 0.15);
    color: #ffffff;
  }

  .deconstruct-demand-link-select .dropdown-option-item.selected {
    background: rgba(99, 102, 241, 0.35);
    color: #ffffff;
    font-weight: 600;
  }

  .arrow-icon {
    font-size: 0.55rem;
    color: #818cf8;
    transition: transform 0.2s;
  }

  .arrow-icon.open {
    transform: rotate(180deg);
  }

  .btn-sync-kanban {
    background: rgba(16, 185, 129, 0.12);
    border: 1px solid rgba(16, 185, 129, 0.3);
    color: #34d399;
    border-radius: 6px;
    padding: 4px 12px;
    font-size: 0.72rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .btn-sync-kanban:hover:not(:disabled) {
    background: rgba(16, 185, 129, 0.22);
    border-color: rgba(16, 185, 129, 0.5);
    box-shadow: 0 0 8px rgba(16, 185, 129, 0.15);
  }

  .btn-sync-kanban:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
