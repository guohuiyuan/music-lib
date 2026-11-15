package qq

func (q *QQ) IsVipAccount() (bool, error) {
	if q.isVipCache != nil {
		return *q.isVipCache, nil
	}
	if q.cookie == "" {
		isVip := false
		q.isVipCache = &isVip
		return false, nil
	}

	options, err := q.AvailableQualityOptions()
	if err != nil {
		return false, err
	}
	isVip := false
	for _, option := range options {
		if option.VIP {
			isVip = true
			break
		}
	}
	q.isVipCache = &isVip
	return isVip, nil
}
