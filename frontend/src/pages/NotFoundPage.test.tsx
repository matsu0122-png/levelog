import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { NotFoundPage } from './NotFoundPage'

describe('NotFoundPage', () => {
  it('renders a 404 message with a link back home', () => {
    render(
      <MemoryRouter>
        <NotFoundPage />
      </MemoryRouter>,
    )

    expect(screen.getByText('404')).toBeInTheDocument()
    expect(screen.getByText('ページが見つかりません')).toBeInTheDocument()
    const link = screen.getByRole('link', { name: 'ホームに戻る' })
    expect(link).toHaveAttribute('href', '/')
  })
})
