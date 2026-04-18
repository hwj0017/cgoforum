import { useContext } from 'react'
import { ForumContext } from './ForumProvider'

export function useForum() {
    const ctx = useContext(ForumContext)
    if (!ctx) {
        throw new Error('useForum must be used within ForumProvider')
    }
    return ctx
}
