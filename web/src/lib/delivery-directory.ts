export interface DeliveryAssigneeOption {
  value: string;
  label: string;
  department?: string;
  aliases?: string[];
}

export interface DeliveryProjectOption {
  project_key: string;
  project_name: string;
}

export interface DeliveryDirectory {
  assignees: DeliveryAssigneeOption[];
  projects: DeliveryProjectOption[];
}

function clean(value: unknown): string {
  return String(value ?? '').trim();
}

export async function fetchDeliveryDirectory(): Promise<DeliveryDirectory> {
  const response = await fetch('/api/delivery/directory', { cache: 'no-store' });
  if (!response.ok) throw new Error(`交付目录加载失败 (${response.status})`);
  const payload = await response.json();

  const seenAssignees = new Set<string>();
  const assignees = (Array.isArray(payload?.assignees) ? payload.assignees : [])
    .map((option: any) => ({
      value: clean(option?.value),
      label: clean(option?.label || option?.value),
      department: clean(option?.department),
      aliases: (Array.isArray(option?.aliases) ? option.aliases : []).map(clean).filter(Boolean)
    }))
    .filter((option: DeliveryAssigneeOption) => {
      const key = option.value.toLocaleLowerCase('zh-CN');
      if (!key || seenAssignees.has(key)) return false;
      seenAssignees.add(key);
      return true;
    })
    .sort((a: DeliveryAssigneeOption, b: DeliveryAssigneeOption) => a.label.localeCompare(b.label, 'zh-CN'));

  const seenProjects = new Set<string>();
  const projects = (Array.isArray(payload?.projects) ? payload.projects : [])
    .map((project: any) => ({
      project_key: clean(project?.project_key).toUpperCase(),
      project_name: clean(project?.project_name || project?.project_key)
    }))
    .filter((project: DeliveryProjectOption) => {
      if (!project.project_key || seenProjects.has(project.project_key)) return false;
      seenProjects.add(project.project_key);
      return true;
    })
    .sort((a: DeliveryProjectOption, b: DeliveryProjectOption) =>
      a.project_name.localeCompare(b.project_name, 'zh-CN') || a.project_key.localeCompare(b.project_key)
    );

  return { assignees, projects };
}
