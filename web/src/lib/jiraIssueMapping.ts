import type {Mapping, Repo} from './jiraIssueWorkspace';
export function mappingError(rows:Mapping[], projects:{key:string}[], repos:Repo[]):string {
 if(rows.length>100)return '最多配置 100 条映射。';
 if(rows.reduce((sum,row)=>sum+new Set(row.task_keys).size,0)>1000)return '映射任务编号合计最多 1000 个。';
 const signatures=new Set<string>();
 for (const [index,row] of rows.entries()) {
  const prefix=`映射 ${index+1}：`;
  if (!projects.some(p=>p.key===row.project)) return prefix+'请选择有效的 Jira 项目。';
  if (!row.repo_ids.length || row.repo_ids.length>20) return prefix+'请选择 1–20 个关联仓库。';
  if (row.repo_ids.some(id=>!repos.some(r=>String(r.id)===id))) return prefix+'仓库已不在当前目录，请重新选择。';
  if (row.task_keys.length>100 || row.task_keys.some(key=>(key.length>64 || !key.startsWith(row.project+'-') || !/^[1-9][0-9]*$/.test(key.slice(row.project.length+1))))) return prefix+'任务编号必须属于所选项目，每条最多 100 个。';
  const signature=JSON.stringify([row.project,[...new Set(row.repo_ids)].sort(),[...new Set(row.task_keys)].sort()]);
  if (signatures.has(signature)) return prefix+'与已有映射重复。';
  signatures.add(signature);
 }
 return '';
}
export function mergeMappings(existing:Mapping[], incoming:Mapping[]):Mapping[] {
 const result=existing.map(m=>({project:m.project,repo_ids:[...m.repo_ids],task_keys:[...m.task_keys]}));
 for (const row of incoming) {
  const tasks=[...new Set(row.task_keys)].sort();
  const match=result.find(m=>m.project===row.project && JSON.stringify([...new Set(m.task_keys)].sort())===JSON.stringify(tasks));
  const combined=[...new Set([...(match?.repo_ids||[]),...row.repo_ids])];
  if (match && combined.length<=20) match.repo_ids=combined;
  else if (!result.some(m=>m.project===row.project && JSON.stringify([...m.repo_ids].sort())===JSON.stringify([...row.repo_ids].sort()) && JSON.stringify([...m.task_keys].sort())===JSON.stringify(tasks))) result.push({project:row.project,repo_ids:[...new Set(row.repo_ids)],task_keys:tasks});
 }
 return result;
}
