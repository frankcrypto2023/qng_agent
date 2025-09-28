import React, { useEffect, useRef, useState } from 'react'
import { CheckCircle, XCircle, Loader2, Play, Pause } from 'lucide-react'

// 工作流节点状态类型
export type NodeStatus = 'Pending' | 'Executing' | 'Success' | 'Failure'

// 工作流节点接口
export interface WorkflowNode {
  id: string
  label: string
  type: string
  status: NodeStatus
  data?: any
  error?: string
}

// 工作流边接口
export interface WorkflowEdge {
  source: string
  target: string
}

// 工作流图谱接口
export interface WorkflowGraph {
  nodes: WorkflowNode[]
  edges: WorkflowEdge[]
}

// 工作流可视化组件属性
interface WorkflowVisualizationProps {
  workflowId?: string
  graph?: WorkflowGraph
  isVisible: boolean
  onNodeClick?: (node: WorkflowNode) => void
  onClose?: () => void
}

// 节点状态样式映射
const getNodeStyles = (status: NodeStatus) => {
  const baseStyles = "rounded-lg p-4 border-2 transition-all duration-300 min-w-[150px] text-center"
  
  switch (status) {
    case 'Pending':
      return `${baseStyles} bg-gray-100 border-gray-300 text-gray-700`
    case 'Executing':
      return `${baseStyles} bg-blue-100 border-blue-400 text-blue-800 animate-pulse`
    case 'Success':
      return `${baseStyles} bg-green-100 border-green-400 text-green-800`
    case 'Failure':
      return `${baseStyles} bg-red-100 border-red-400 text-red-800`
    default:
      return `${baseStyles} bg-gray-100 border-gray-300 text-gray-700`
  }
}

// 状态图标组件
const StatusIcon: React.FC<{ status: NodeStatus }> = ({ status }) => {
  switch (status) {
    case 'Executing':
      return <Loader2 className="w-4 h-4 animate-spin" />
    case 'Success':
      return <CheckCircle className="w-4 h-4 text-green-600" />
    case 'Failure':
      return <XCircle className="w-4 h-4 text-red-600" />
    default:
      return <Play className="w-4 h-4 text-gray-500" />
  }
}

// 工作流可视化组件
export const WorkflowVisualization: React.FC<WorkflowVisualizationProps> = ({
  workflowId,
  graph,
  isVisible,
  onNodeClick,
  onClose
}) => {
  const [currentGraph, setCurrentGraph] = useState<WorkflowGraph | null>(null)
  const [isExpanded, setIsExpanded] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  // 更新图谱数据
  useEffect(() => {
    if (graph) {
      setCurrentGraph(graph)
    }
  }, [graph])

  // 调试信息
  console.log('WorkflowVisualization render:', {
    isVisible,
    hasGraph: !!currentGraph,
    workflowId,
    graph: currentGraph
  })

  // 如果没有图谱数据或不可见，不渲染
  if (!isVisible || !currentGraph) {
    console.log('WorkflowVisualization not rendering:', { isVisible, hasGraph: !!currentGraph })
    return null
  }

  // 计算节点位置（简单的网格布局）
  const calculateNodePositions = (nodes: WorkflowNode[], edges: WorkflowEdge[]) => {
    const positions: Record<string, { x: number; y: number }> = {}
    const nodeMap = new Map(nodes.map(node => [node.id, node]))
    
    // 简单的拓扑排序来确定层级
    const levels: string[][] = []
    const visited = new Set<string>()
    const inDegree = new Map<string, number>()
    
    // 计算入度
    nodes.forEach(node => inDegree.set(node.id, 0))
    edges.forEach(edge => {
      inDegree.set(edge.target, (inDegree.get(edge.target) || 0) + 1)
    })
    
    // 找到所有入度为0的节点（起始节点）
    const startNodes = nodes.filter(node => inDegree.get(node.id) === 0)
    if (startNodes.length > 0) {
      levels.push(startNodes.map(node => node.id))
      startNodes.forEach(node => visited.add(node.id))
    }
    
    // 继续处理其他层级
    while (visited.size < nodes.length) {
      const currentLevel: string[] = []
      nodes.forEach(node => {
        if (!visited.has(node.id)) {
          const incomingEdges = edges.filter(edge => edge.target === node.id)
          const allDependenciesVisited = incomingEdges.every(edge => visited.has(edge.source))
          if (allDependenciesVisited) {
            currentLevel.push(node.id)
          }
        }
      })
      
      if (currentLevel.length === 0) break
      
      levels.push(currentLevel)
      currentLevel.forEach(nodeId => visited.add(nodeId))
    }
    
    // 为每个层级的节点分配位置
    levels.forEach((levelNodes, levelIndex) => {
      levelNodes.forEach((nodeId, nodeIndex) => {
        const x = nodeIndex * 280 + 100  // 增加左侧间隔和节点间距
        const y = levelIndex * 180 + 120
        positions[nodeId] = { x, y }
      })
    })
    
    return positions
  }

  const nodePositions = calculateNodePositions(currentGraph.nodes, currentGraph.edges)

  return (
    <div 
      ref={containerRef}
      className={`fixed right-0 top-0 h-screen bg-white border-l border-gray-200 shadow-lg transition-all duration-300 z-40 ${
        isExpanded ? 'w-[600px]' : 'w-[500px]'
      }`}
    >
      {/* 头部 */}
      <div className="p-4 border-b border-gray-200 bg-gray-50">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-800">工作流执行</h3>
        <div className="flex items-center space-x-2">
            <button
              onClick={() => setIsExpanded(!isExpanded)}
              className="p-1 rounded hover:bg-gray-200 transition-colors"
              title={isExpanded ? '收起' : '展开'}
            >
              {isExpanded ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4" />}
            </button>
            {onClose && (
              <button
                onClick={onClose}
                className="p-1 rounded hover:bg-red-200 transition-colors"
                title="关闭工作流面板"
              >
                <XCircle className="w-4 h-4 text-red-600" />
            </button>
          )}
          </div>
        </div>
        {workflowId && (
          <p className="text-sm text-gray-500 mt-1">ID: {workflowId}</p>
        )}
      </div>

      {/* 工作流图谱 */}
      <div className="flex-1 overflow-auto p-4">
        <div className="relative" style={{ height: 'calc(100vh - 200px)', width: '100%' }}>
          {/* 渲染节点 */}
          {currentGraph.nodes.map((node) => {
            const position = nodePositions[node.id]
            if (!position) return null

            return (
              <div
                key={node.id}
                className={`absolute cursor-pointer ${getNodeStyles(node.status)}`}
                style={{
                  left: position.x,
                  top: position.y,
                  transform: 'translate(-50%, -50%)',
                  zIndex: 2
                }}
                onClick={() => onNodeClick?.(node)}
                title={node.error || node.label}
              >
                <div className="flex items-center justify-center space-x-2">
                  <StatusIcon status={node.status} />
                  <span className="text-sm font-medium">{node.label}</span>
                </div>
                {node.status === 'Failure' && node.error && (
                  <div className="text-xs text-red-600 mt-1 max-w-[100px] truncate">
                    {node.error}
                  </div>
                )}
              </div>
            )
          })}

          {/* 渲染边（连接线） */}
          <svg
            className="absolute inset-0 pointer-events-none"
            style={{ zIndex: 1, width: '100%', height: '100%' }}
          >
            {/* 箭头标记定义 */}
            <defs>
              <marker
                id="arrowhead"
                markerWidth="10"
                markerHeight="7"
                refX="9"
                refY="3.5"
                orient="auto"
              >
                <polygon
                  points="0 0, 10 3.5, 0 7"
                  fill="#6b7280"
                />
              </marker>
            </defs>
            
            {currentGraph.edges.map((edge, index) => {
              const sourcePos = nodePositions[edge.source]
              const targetPos = nodePositions[edge.target]
              
              if (!sourcePos || !targetPos) return null

              const sourceNode = currentGraph.nodes.find(n => n.id === edge.source)
              const targetNode = currentGraph.nodes.find(n => n.id === edge.target)
              
              if (!sourceNode || !targetNode) return null

              // 计算连接点
              const startX = sourcePos.x
              const startY = sourcePos.y + 40 // 节点底部
              const endX = targetPos.x
              const endY = targetPos.y - 40 // 节点顶部

              // 根据目标节点状态确定线条颜色
              const getLineColor = (status: NodeStatus) => {
                switch (status) {
                  case 'Success':
                    return '#10b981' // green-500
                  case 'Failure':
                    return '#ef4444' // red-500
                  case 'Executing':
                    return '#3b82f6' // blue-500
                  default:
                    return '#6b7280' // gray-500
                }
              }

              return (
                <line
                  key={index}
                  x1={startX}
                  y1={startY}
                  x2={endX}
                  y2={endY}
                  stroke={getLineColor(targetNode.status)}
                  strokeWidth="3"
                  markerEnd="url(#arrowhead)"
                />
              )
            })}
          </svg>
        </div>
      </div>

      {/* 状态统计 */}
      <div className="p-4 border-t border-gray-200 bg-gray-50">
        <div className="grid grid-cols-2 gap-2 text-sm">
          <div className="flex items-center space-x-1">
            <div className="w-3 h-3 bg-gray-400 rounded-full"></div>
            <span>待执行: {currentGraph.nodes.filter(n => n.status === 'Pending').length}</span>
          </div>
          <div className="flex items-center space-x-1">
            <div className="w-3 h-3 bg-blue-400 rounded-full"></div>
            <span>执行中: {currentGraph.nodes.filter(n => n.status === 'Executing').length}</span>
          </div>
          <div className="flex items-center space-x-1">
            <div className="w-3 h-3 bg-green-400 rounded-full"></div>
            <span>成功: {currentGraph.nodes.filter(n => n.status === 'Success').length}</span>
          </div>
          <div className="flex items-center space-x-1">
            <div className="w-3 h-3 bg-red-400 rounded-full"></div>
            <span>失败: {currentGraph.nodes.filter(n => n.status === 'Failure').length}</span>
          </div>
        </div>
      </div>
    </div>
  )
}

export default WorkflowVisualization