import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatTimestamp(date: Date): string {
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMinutes = Math.floor(diffMs / (1000 * 60))
  const diffHours = Math.floor(diffMinutes / 60)
  const diffDays = Math.floor(diffHours / 24)

  if (diffMinutes < 1) return 'Just now'
  if (diffMinutes < 60) return `${diffMinutes}m ago`
  if (diffHours < 24) return `${diffHours}h ago`
  if (diffDays < 7) return `${diffDays}d ago`
  
  return date.toLocaleDateString()
}

export function generateId(): string {
  return Math.random().toString(36).substring(2) + Date.now().toString(36)
}

/**
 * Filters out GPT-OSS Harmony format metadata from message content
 * Removes content between <|channel|> and <|end|> markers
 * @param content The raw message content
 * @returns Filtered content without metadata
 */
export function filterHarmonyMetadata(content: string): string {
  // Pattern to match <|channel|>...<|end|> blocks
  const harmonyPattern = /<\|channel\|>.*?<\|end\|>/gs
  
  // Remove all harmony metadata blocks
  let filtered = content.replace(harmonyPattern, '')
  
  // Clean up any extra whitespace that might be left
  // Replace multiple consecutive newlines with single newline
  filtered = filtered.replace(/\n\s*\n\s*\n/g, '\n\n')
  filtered = filtered.trim()
  
  return filtered
}