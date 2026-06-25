package game

func DefaultTemplates() []Template {
	return []Template{
		{ID: "classic-6", Name: "6 人基础板", Seats: 6, Roles: []Role{RoleWerewolf, RoleWerewolf, RoleSeer, RoleWitch, RoleVillager, RoleVillager}},
		{ID: "classic-7", Name: "7 人基础板", Seats: 7, Roles: []Role{RoleWerewolf, RoleWerewolf, RoleSeer, RoleWitch, RoleVillager, RoleVillager, RoleVillager}},
		{ID: "classic-8", Name: "8 人基础板", Seats: 8, Roles: []Role{RoleWerewolf, RoleWerewolf, RoleWerewolf, RoleSeer, RoleWitch, RoleVillager, RoleVillager, RoleVillager}},
		{ID: "classic-9", Name: "9 人基础板", Seats: 9, Roles: []Role{RoleWerewolf, RoleWerewolf, RoleWerewolf, RoleSeer, RoleWitch, RoleHunter, RoleVillager, RoleVillager, RoleVillager}},
	}
}
