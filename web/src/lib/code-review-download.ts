// Prefer the server's UTF-8 filename; the report document provides a safe fallback.
export function attachmentFilename(header: string | null, fallback: string): string {
 const encoded = header?.match(/filename\*\s*=\s*UTF-8''([^;]+)/i)?.[1];
 const plain = header?.match(/filename\s*=\s*(?:"([^"]+)"|([^;]+))/i);
 let name = plain?.[1] || plain?.[2]?.trim() || fallback;
 if (encoded) { try { name = decodeURIComponent(encoded); } catch { name = fallback; } }
 return name.replace(/[<>:"/\\|?*\x00-\x1f\x7f]/g, "-").replace(/[\u202a-\u202e\u2066-\u2069]/g, "");
}
