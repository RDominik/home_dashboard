import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import App from './App'
import DashboardHome from './pages/DashboardHome'
import EnergyFlow from './pages/EnergyFlow'
import Heating from './pages/Heating'
import Grafana from './pages/Grafana'
import UpdatePage from './pages/UpdatePage'
import GoE from './pages/goE'
import Huehnerklappe from './pages/Huehnerklappe'

const router = createBrowserRouter([
  {
    path: '/',
    element: <App />,
    children: [
      { index: true, element: <DashboardHome /> },
      { path: 'energy', element: <EnergyFlow /> },
      { path: 'heating', element: <Heating /> },
      { path: 'grafana', element: <Grafana /> },
      { path: 'huehnerklappe', element: <Huehnerklappe /> },
      { path: 'goE', element: <GoE /> },
      { path: 'update', element: <UpdatePage /> }
    ],
  },
])

const rootElement = document.getElementById('root')

if (!rootElement) {
  throw new Error('Root element not found')
}

createRoot(rootElement).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>
)
