import React from 'react'
import { createRoot } from 'react-dom/client'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import App from './App.jsx'
import DashboardHome from './pages/DashboardHome.jsx'
import EnergyFlow from './pages/EnergyFlow.jsx'
import Heating from './pages/Heating.jsx'
import Grafana from './pages/Grafana.jsx'
import UpdatePage from './pages/UpdatePage.jsx'
import GoE from './pages/goE.jsx'
import Huehnerklappe from './pages/Huehnerklappe.jsx'

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

createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>
)
