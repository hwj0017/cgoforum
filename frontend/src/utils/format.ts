export function formatTime(value?: string): string {
    if (!value) {
        return '-'
    }
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) {
        return '-'
    }
    return date.toLocaleString()
}

export function shortText(text?: string, max = 120): string {
    if (!text) {
        return ''
    }
    if (text.length <= max) {
        return text
    }
    return `${text.slice(0, max)}...`
}
