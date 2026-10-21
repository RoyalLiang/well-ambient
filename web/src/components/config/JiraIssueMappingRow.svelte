<script lang="ts">
 import Button from '../shared/Button.svelte';
 import Select from '../shared/Select.svelte';
 import MultiSelect from '../shared/MultiSelect.svelte';
 import TextInput from '../shared/TextInput.svelte';
 import { taskKeys, type Mapping, type Repo } from '../../lib/jiraIssueWorkspace';
 export let mapping:Mapping;
 export let index:number;
 export let projects:{key:string;name:string}[]=[];
 export let repos:Repo[]=[];
 export let disabled=false;
 export let onChange:(value:Mapping)=>void=()=>{};
 export let onRemove:()=>void=()=>{};
 let keys='';
 $: keys=mapping.task_keys.join(', ');
 $: projectOptions=projects.map(p=>({value:p.key,label:p.key,meta:p.name}));
 $: repoOptions=repos.map(r=>({value:String(r.id),label:r.path_with_namespace}));
 function changeTasks(e:Event){onChange({...mapping,task_keys:taskKeys((e.target as HTMLInputElement).value)})}
</script>
<div class="mapping-row" aria-label={`项目映射 ${index+1}`}>
 <div class="project"><Select id={`sync-project-${index}`} label="Jira 项目" value={mapping.project} options={projectOptions} {disabled} on:change={e=>onChange({...mapping,project:e.detail,task_keys:[]})} /></div>
 <div class="repositories"><MultiSelect id={`sync-repos-${index}`} label="关联仓库" values={mapping.repo_ids} options={repoOptions} {disabled} appearance="settings" shadowless on:change={e=>onChange({...mapping,repo_ids:e.detail})} /></div>
 <div class="tasks" on:change={changeTasks}><TextInput id={`sync-tasks-${index}`} label="任务编号" value={keys} placeholder="留空同步项目全部任务" {disabled} /></div>
 <Button variant="ghost" ariaLabel={`删除映射 ${index+1}`} {disabled} on:click={onRemove}>删除</Button>
</div>
<style>
 .mapping-row{display:grid;grid-template-columns:minmax(120px,.7fr) minmax(180px,1.4fr) minmax(180px,1.2fr) auto;align-items:end;gap:16px;padding:18px 0;border-bottom:1px solid var(--wa-line,#e4e9e8)}
 .mapping-row>div{min-width:0}.mapping-row :global(.input-group){margin-bottom:0}.mapping-row :global(.select-inline-input){border:0!important;background:transparent!important;box-shadow:none!important}
 @container (max-width:900px){.mapping-row{grid-template-columns:minmax(0,1fr) minmax(0,1fr)}.tasks{grid-column:1}.mapping-row>:global(button){justify-self:end}}
 @container (max-width:600px){.mapping-row{grid-template-columns:minmax(0,1fr)}.tasks{grid-column:auto}}
</style>
