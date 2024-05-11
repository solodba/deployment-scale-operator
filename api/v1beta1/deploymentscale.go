package v1beta1

// Deployment构造函数
func NewDeployment() *Deployment {
	return &Deployment{}
}

// DeploymentScaleSpec构造函数
func NewDeploymentScaleSpec() *DeploymentScaleSpec {
	return &DeploymentScaleSpec{
		Deployments: make([]*Deployment, 0),
	}
}

// DeploymentScaleSpec结构体添加方法
func (d *DeploymentScaleSpec) AddItems(items ...*Deployment) {
	d.Deployments = append(d.Deployments, items...)
}

// DeploymentScaleStatus构造函数
func NewDeploymentScaleStatus() *DeploymentScaleStatus {
	return &DeploymentScaleStatus{}
}

// DeploymentScale构造函数
func NewDeploymentScale() *DeploymentScale {
	return &DeploymentScale{
		Spec:   *NewDeploymentScaleSpec(),
		Status: *NewDeploymentScaleStatus(),
	}
}

// DeploymentScaleList构造函数
func NewDeploymentScaleList() *DeploymentScaleList {
	return &DeploymentScaleList{
		Items: make([]DeploymentScale, 0),
	}
}

// DeploymentScaleList添加方法
func (d *DeploymentScaleList) AddItems(items ...DeploymentScale) {
	d.Items = append(d.Items, items...)
}
